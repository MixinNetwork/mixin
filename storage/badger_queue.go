package storage

import (
	"encoding/binary"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/dgraph-io/badger"
)

func (s *BadgerStore) QueueAppendSnapshot(snap *common.Snapshot) error {
	txn := s.cacheDB.NewTransaction(true)
	defer txn.Discard()

	seq, err := cacheQueueNextSeq(txn)
	if err != nil {
		return err
	}
	key := cacheSnapshotQueueKey(seq)
	val := common.MsgpackMarshalPanic(snap)
	err = txn.SetWithTTL(key, val, config.CacheTTL)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) QueuePollSnapshots(offset uint64, hook func(k uint64, v []byte) error) error {
	txn := s.cacheDB.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	key := cacheSnapshotQueueKey(offset)
	prefix := []byte(cachePrefixSnapshotQueue)
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
	}
	return nil
}

func cacheQueueNextSeq(txn *badger.Txn) (uint64, error) {
	var seq uint64
	key := []byte(cacheKeyAllQueueSeq)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		seq = uint64(time.Now().UnixNano())
	} else if err != nil {
		return 0, err
	} else {
		v, err := item.ValueCopy(nil)
		if err != nil {
			return 0, err
		}
		seq = binary.BigEndian.Uint64(v)
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, seq+1)
	return seq, txn.Set(key, buf)
}

func cacheSnapshotQueueKey(offset uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(offset))
	return append([]byte(cachePrefixSnapshotQueue), buf...)
}
