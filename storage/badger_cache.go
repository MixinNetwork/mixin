package storage

import (
	"encoding/binary"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

const (
	cachePrefixTransactionQueue  = "CACHETRANSACTIONQUEUE"
	cachePrefixTransactionOrder  = "CACHETRANSACTIONORDER"
	cachePrefixTransactionCache  = "CACHETRANSACTIONPAYLOAD"
	cachePrefixSnapshotNodeQueue = "SNAPSHOTNODEQUEUE"
	cachePrefixSnapshotNodeMeta  = "SNAPSHOTNODEMETA"
)

func (s *BadgerStore) CacheRetrieveTransactions(limit int) ([]*common.VersionedTransaction, error) {
	var txs []*common.VersionedTransaction
	err := s.cacheDB.Update(func(txn *badger.Txn) error {
		prefix := []byte(cachePrefixTransactionQueue)
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		var processed [][]byte
		var hash crypto.Hash
		filter := make(map[crypto.Hash]bool)
		it.Seek(cacheTransactionQueueKey(0, hash))
		for ; len(txs) < limit && it.Valid(); it.Next() {
			key := it.Item().KeyCopy(nil)
			copy(hash[:], key[len(cachePrefixTransactionQueue)+8:])
			processed = append(processed, key)
			processed = append(processed, cacheTransactionOrderKey(hash))
			if filter[hash] {
				continue
			}
			filter[hash] = true
			ver, err := s.cacheReadTransaction(txn, hash)
			if err != nil {
				return err
			}
			if ver != nil {
				txs = append(txs, ver)
			}
		}

		for _, k := range processed {
			err := txn.Delete(k)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return txs, err
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
				key = cacheTransactionOrderKey(hashes[i])
				err = txn.Delete(key)
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

	hash := tx.PayloadHash()
	key := cacheTransactionOrderKey(hash)
	_, err := txn.Get(key)
	if err == nil {
		return nil
	}
	etr := badger.NewEntry(key, []byte{}).WithTTL(time.Duration(s.custom.Node.CacheTTL) * time.Second)
	err = txn.SetEntry(etr)
	if err != nil {
		return err
	}

	key = cacheTransactionCacheKey(hash)
	val := tx.CompressMarshal()
	etr = badger.NewEntry(key, val).WithTTL(time.Duration(s.custom.Node.CacheTTL+60) * time.Second)
	err = txn.SetEntry(etr)
	if err != nil {
		return err
	}

	key = cacheTransactionQueueKey(uint64(time.Now().UnixNano()), hash)
	etr = badger.NewEntry(key, []byte{}).WithTTL(time.Duration(s.custom.Node.CacheTTL) * time.Second)
	err = txn.SetEntry(etr)
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (s *BadgerStore) CacheGetTransaction(hash crypto.Hash) (*common.VersionedTransaction, error) {
	txn := s.cacheDB.NewTransaction(false)
	defer txn.Discard()

	return s.cacheReadTransaction(txn, hash)
}

func (s *BadgerStore) cacheReadTransaction(txn *badger.Txn, tx crypto.Hash) (*common.VersionedTransaction, error) {
	key := cacheTransactionCacheKey(tx)
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

func cacheTransactionQueueKey(ts uint64, hash crypto.Hash) []byte {
	key := []byte(cachePrefixTransactionQueue)
	key = binary.BigEndian.AppendUint64(key, ts)
	return append(key, hash[:]...)
}

func cacheTransactionOrderKey(hash crypto.Hash) []byte {
	return append([]byte(cachePrefixTransactionOrder), hash[:]...)
}
