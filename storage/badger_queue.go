package storage

import (
	"encoding/binary"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

func (s *BadgerStore) QueueAppendSnapshot(peerId crypto.Hash, snap *common.Snapshot) error {
	txn := s.cacheDB.NewTransaction(true)
	defer txn.Discard()

	seq := uint64(time.Now().UnixNano())
	key := cacheSnapshotQueueKey(seq)
	val := common.MsgpackMarshalPanic(snap)
	val = append(peerId[:], val...)
	err := txn.SetWithTTL(key, val, config.CacheTTL)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) QueuePollSnapshots(offset uint64, hook func(offset uint64, peerId crypto.Hash, snap *common.Snapshot) error) error {
	txn := s.cacheDB.NewTransaction(true)
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
		off := binary.BigEndian.Uint64(k[len(cachePrefixSnapshotQueue):])
		var peerId crypto.Hash
		copy(peerId[:], v[:len(peerId)])
		var snap common.Snapshot
		err = msgpack.Unmarshal(v[len(peerId):], &snap)
		if err != nil {
			return err
		}
		err = hook(off, peerId, &snap)
		if err != nil {
			return err
		}
		err = txn.Delete(k)
		if err != nil {
			return err
		}
	}
	it.Close()
	return txn.Commit()
}

func cacheSnapshotQueueKey(offset uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(offset))
	return append([]byte(cachePrefixSnapshotQueue), buf...)
}
