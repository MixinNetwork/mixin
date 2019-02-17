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

func (s *BadgerStore) QueueRemoveSnapshot(seq uint64, hash crypto.Hash) error {
	txn := s.cacheDB.NewTransaction(true)
	defer txn.Discard()

	key := cacheSnapshotCacheKey(hash)
	err := txn.Delete(key)
	if err != nil {
		return err
	}
	key = cacheSnapshotQueueKey(seq)
	err = txn.Delete(key)
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (s *BadgerStore) QueueAppendSnapshot(peerId crypto.Hash, snap *common.Snapshot) error {
	txn := s.cacheDB.NewTransaction(true)
	defer txn.Discard()

	key := cacheSnapshotCacheKey(snap.PayloadHash())
	_, err := txn.Get(key)
	if err == nil {
		return nil
	} else if err != badger.ErrKeyNotFound {
		return err
	}
	err = txn.SetWithTTL(key, []byte{}, config.CacheTTL)
	if err != nil {
		return err
	}

	seq := uint64(time.Now().UnixNano())
	key = cacheSnapshotQueueKey(seq)
	val := common.MsgpackMarshalPanic(snap)
	val = append(peerId[:], val...)
	err = txn.SetWithTTL(key, val, config.CacheTTL)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) QueuePollSnapshots(offset uint64, hook func(seq uint64, peerId crypto.Hash, snap *common.Snapshot) error) error {
	txn := s.cacheDB.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	count := 0
	key := cacheSnapshotQueueKey(offset)
	prefix := []byte(cachePrefixSnapshotQueue)
	for it.Seek(key); it.ValidForPrefix(prefix) && count < 100; it.Next() {
		count = count + 1
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
	}
	return nil
}

func cacheSnapshotQueueKey(offset uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(offset))
	return append([]byte(cachePrefixSnapshotQueue), buf...)
}
