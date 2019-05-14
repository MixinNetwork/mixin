package storage

import (
	"encoding/binary"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/VictoriaMetrics/fastcache"
	"github.com/dgraph-io/badger"
)

type Queue struct {
	db        *badger.DB
	finalRing *RingBuffer
	cacheRing *RingBuffer
	cache     *fastcache.Cache
	order     uint64
}

type PeerSnapshot struct {
	key      []byte
	PeerId   crypto.Hash
	Snapshot *common.Snapshot
}

func (ps *PeerSnapshot) finalKey() []byte {
	hash := ps.Snapshot.PayloadHash().ForNetwork(ps.PeerId)
	return []byte("BADGER:QUEUE:FINAL:" + hash.String())
}

func (ps *PeerSnapshot) cacheKey() []byte {
	hash := ps.Snapshot.PayloadHash()
	hash = hash.ForNetwork(ps.PeerId)
	for _, sig := range ps.Snapshot.Signatures {
		hash = crypto.NewHash(append(hash[:], sig[:]...))
	}
	return []byte("BADGER:QUEUE:CACHE:" + hash.String())
}

func NewQueue(db *badger.DB, cache *fastcache.Cache) *Queue {
	q := &Queue{
		db:        db,
		cache:     cache,
		cacheRing: NewRingBuffer(1024 * 1024),
		finalRing: NewRingBuffer(1024 * 16),
	}
	q.order = q.snapshotQueueOrder()
	go func() {
		for {
			ps, err := q.PopFinal()
			if err != nil {
				logger.Println("NewQueue PopFinal", err)
			}
			if ps == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			err = q.writeFinal(ps)
			if err != nil {
				logger.Println("NewQueue PopFinal", err)
			}
		}
	}()
	return q
}

func (q *Queue) Dispose() {
	q.finalRing.Dispose()
	q.cacheRing.Dispose()
}

func (q *Queue) PutFinal(ps *PeerSnapshot) error {
	ps.key = ps.finalKey()
	data := q.cache.Get(nil, ps.key)
	if len(data) > 0 {
		return nil
	}

	q.cache.Set(ps.key, []byte{0})
	for {
		put, err := q.finalRing.Offer(ps)
		if err != nil || put {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func (q *Queue) PopFinal() (*PeerSnapshot, error) {
	item, err := q.finalRing.Poll(false)
	if err != nil || item == nil {
		return nil, err
	}
	ps := item.(*PeerSnapshot)
	q.cache.Del(ps.key)
	return ps, nil
}

func (q *Queue) PutCache(ps *PeerSnapshot) error {
	ps.key = ps.cacheKey()
	data := q.cache.Get(nil, ps.key)
	if len(data) > 0 {
		return nil
	}

	q.cache.Set(ps.key, []byte{0})
	for {
		put, err := q.cacheRing.Offer(ps)
		if err != nil || put {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func (q *Queue) PopCache() (*PeerSnapshot, error) {
	item, err := q.cacheRing.Poll(false)
	if err != nil || item == nil {
		return nil, err
	}
	ps := item.(*PeerSnapshot)
	q.cache.Del(ps.key)
	return ps, nil
}

func (s *BadgerStore) QueueInfo() (uint64, uint64, uint64, error) {
	txn := s.cacheDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	it := txn.NewIterator(opts)
	defer it.Close()

	var count uint64
	for it.Rewind(); it.Valid(); it.Next() {
		count = count + 1
	}
	return count, s.queue.snapshotQueueLen(), s.queue.cacheRing.Len(), nil
}

func (s *BadgerStore) QueueAppendSnapshot(peerId crypto.Hash, snap *common.Snapshot, finalized bool) error {
	ps := &PeerSnapshot{
		PeerId:   peerId,
		Snapshot: snap,
	}
	if finalized {
		return s.queue.PutFinal(ps)
	}
	return s.queue.PutCache(ps)
}

func (s *BadgerStore) QueuePollSnapshots(hook func(peerId crypto.Hash, snap *common.Snapshot) error) {
	limit := 100
	for !s.closing {
		ps, err := s.queue.PopCache()
		if err != nil {
			logger.Println("QueuePollSnapshots PopCache", err)
		}
		if ps != nil {
			hook(ps.PeerId, ps.Snapshot)
		}
		snapshots, err := s.queue.batchRetrieveSnapshots(limit)
		if err != nil {
			logger.Println("QueuePollSnapshots batchRetrieveSnapshots", err)
		}
		for _, ps := range snapshots {
			hook(ps.PeerId, ps.Snapshot)
		}
		if len(snapshots) < limit && ps == nil {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (q *Queue) writeFinal(ps *PeerSnapshot) error {
	return q.db.Update(func(txn *badger.Txn) error {
		q.order = q.order + 1
		key := cacheSnapshotQueueKey(q.order)
		val := common.MsgpackMarshalPanic(ps)
		return txn.Set(key, val)
	})
}

func (q *Queue) batchRetrieveSnapshots(limit int) ([]*PeerSnapshot, error) {
	orders := make([]uint64, 0)
	snapshots := make([]*PeerSnapshot, 0)
	err := q.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(cachePrefixSnapshotQueue)
		it.Seek(cacheSnapshotQueueKey(0))
		for ; it.ValidForPrefix(prefix) && len(snapshots) < limit; it.Next() {
			item := it.Item()
			v, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			var ps PeerSnapshot
			err = common.MsgpackUnmarshal(v, &ps)
			if err != nil {
				return err
			}
			ps.Snapshot.Hash = ps.Snapshot.PayloadHash()
			snapshots = append(snapshots, &ps)
			orders = append(orders, cacheSnapshotQueueOrder(item.Key()))
		}
		return nil
	})
	if err != nil {
		return snapshots, err
	}

	wb := q.db.NewWriteBatch()
	defer wb.Cancel()
	for _, order := range orders {
		key := cacheSnapshotQueueKey(order)
		err := wb.Delete(key)
		if err != nil {
			return snapshots, err
		}
	}
	return snapshots, wb.Flush()
}

func (q *Queue) snapshotQueueLen() uint64 {
	var sequence uint64

	txn := q.db.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = false

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(cacheSnapshotQueueKey(uint64(0)))
	if it.ValidForPrefix([]byte(cachePrefixSnapshotQueue)) {
		item := it.Item()
		sequence = cacheSnapshotQueueOrder(item.Key())
	}
	return q.snapshotQueueOrder() - sequence
}

func (q *Queue) snapshotQueueOrder() uint64 {
	var sequence uint64

	txn := q.db.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(cacheSnapshotQueueKey(^uint64(0)))
	if it.ValidForPrefix([]byte(cachePrefixSnapshotQueue)) {
		item := it.Item()
		sequence = cacheSnapshotQueueOrder(item.Key()) + 1
	}
	return sequence
}

func cacheSnapshotQueueKey(order uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, order)
	return append([]byte(cachePrefixSnapshotQueue), buf...)
}

func cacheSnapshotQueueOrder(key []byte) uint64 {
	order := key[len(cachePrefixSnapshotQueue):]
	return binary.BigEndian.Uint64(order)
}
