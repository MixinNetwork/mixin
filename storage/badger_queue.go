package storage

import (
	"encoding/binary"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

const queuePrefixTX = "TX"

func (s *BadgerStore) QueueAdd(tx *common.SignedTransaction) error {
	return s.queueDB.Update(func(txn *badger.Txn) error {
		ival, err := msgpack.Marshal(tx)
		if err != nil {
			return err
		}
		key := queueTxKey(uint64(time.Now().UnixNano())) // FIXME NTP time may not monotonic increase
		return txn.Set([]byte(key), ival)
	})
}

func (s *BadgerStore) QueuePoll(offset uint64, hook func(k uint64, v []byte) error) error {
	return s.queueDB.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(queueTxKey(offset)); it.ValidForPrefix([]byte(queuePrefixTX)); it.Next() {
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

func queueTxKey(offset uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(offset))
	return append([]byte(queuePrefixTX), buf...)
}
