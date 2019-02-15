package storage

import (
	"encoding/binary"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

const (
	cachePrefixTransactionQueue = "TRANSACTIONQUEUE"
)

func (s *BadgerStore) CacheAppendTransactionToQueue(tx *common.SignedTransaction) error {
	return s.cacheDB.Update(func(txn *badger.Txn) error {
		ival, err := msgpack.Marshal(tx)
		if err != nil {
			return err
		}
		key := cacheTransactionQueueKey(uint64(time.Now().UnixNano())) // FIXME NTP time may not monotonic increase
		return txn.Set(key, ival)
	})
}

func (s *BadgerStore) CachePollTransactionsQueue(offset uint64, hook func(k uint64, v []byte) error) error {
	return s.cacheDB.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		key := cacheTransactionQueueKey(offset)
		prefix := []byte(cachePrefixTransactionQueue)
		for it.Seek(key); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := item.Key()
			v, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			err = hook(binary.BigEndian.Uint64(k[2:]), v)
			if err != nil {
				return err
			}
			err = txn.Delete(k)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func cacheTransactionQueueKey(offset uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(offset))
	return append([]byte(cachePrefixTransactionQueue), buf...)
}
