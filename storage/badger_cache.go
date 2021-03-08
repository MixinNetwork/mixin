package storage

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v2"
)

const (
	cachePrefixTransactionCache  = "TRANSACTIONCACHE"
	cachePrefixSnapshotNodeQueue = "SNAPSHOTNODEQUEUE"
	cachePrefixSnapshotNodeMeta  = "SNAPSHOTNODEMETA"
)

func (s *BadgerStore) CacheListTransactions(hook func(tx *common.VersionedTransaction) error) error {
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
		key := it.Item().KeyCopy(nil)[len(prefix):]
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
		ver, err := common.DecompressUnmarshalVersionedTransaction(v)
		if err != nil {
			return err
		}
		err = hook(ver)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *BadgerStore) CacheRemoveTransactions(hashes []crypto.Hash) error {
	batch := 100
	for {
		err := s.cacheDB.Update(func(txn *badger.Txn) error {
			for i := range hashes {
				key := cacheTransactionCacheKey(hashes[i])
				err := txn.Delete(key)
				if err != nil {
					return err
				}
				if i == batch {
					break
				}
			}
			return nil
		})
		if err != nil || len(hashes) <= batch {
			return err
		}
		hashes = hashes[batch:]
	}
}

func (s *BadgerStore) CachePutTransaction(tx *common.VersionedTransaction) error {
	txn := s.cacheDB.NewTransaction(true)
	defer txn.Discard()

	key := cacheTransactionCacheKey(tx.PayloadHash())
	val := tx.CompressMarshal()
	etr := badger.NewEntry(key, val).WithTTL(time.Duration(s.custom.Node.CacheTTL) * time.Second * 8)
	err := txn.SetEntry(etr)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) CacheGetTransaction(hash crypto.Hash) (*common.VersionedTransaction, error) {
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
	return common.DecompressUnmarshalVersionedTransaction(val)
}

func cacheTransactionCacheKey(hash crypto.Hash) []byte {
	return append([]byte(cachePrefixTransactionCache), hash[:]...)
}
