package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

const (
	cachePrefixTransactionCache = "TRANSACTIONCACHE"
)

func (s *BadgerStore) CacheListTransactions(hook func(tx *common.SignedTransaction) error) error {
	snapTxn := s.snapshotsDB.NewTransaction(false)
	defer snapTxn.Discard()

	txn := s.cacheDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	it := txn.NewIterator(opts)
	defer it.Close()

	prefix := []byte(cachePrefixTransactionCache)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		key := it.Item().Key()[len(prefix):]
		key = append([]byte(graphPrefixFinalization), key...)
		_, err := snapTxn.Get(key)
		if err == nil {
			continue
		} else if err != badger.ErrKeyNotFound {
			return err
		}

		v, err := it.Item().ValueCopy(nil)
		if err != nil {
			return err
		}
		var tx common.SignedTransaction
		err = msgpack.Unmarshal(v, &tx)
		if err != nil {
			return err
		}
		err = hook(&tx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *BadgerStore) CachePutTransaction(tx *common.SignedTransaction) error {
	txn := s.cacheDB.NewTransaction(true)
	defer txn.Discard()

	key := cacheTransactionCacheKey(tx.PayloadHash())
	val := common.MsgpackMarshalPanic(tx)
	err := txn.SetWithTTL(key, val, config.CacheTTL)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) CacheGetTransaction(hash crypto.Hash) (*common.SignedTransaction, error) {
	txn := s.cacheDB.NewTransaction(false)
	defer txn.Discard()

	key := cacheTransactionCacheKey(hash)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	var tx common.SignedTransaction
	err = msgpack.Unmarshal(val, &tx)
	return &tx, err
}

func cacheTransactionCacheKey(hash crypto.Hash) []byte {
	return append([]byte(cachePrefixTransactionCache), hash[:]...)
}
