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
			var snapshots []*PeerSnapshot
			for len(snapshots) < 100 {
				ps, err := q.PopFinal()
				if err != nil {
					logger.Println("NewQueue PopFinal", err)
					break
				}
				if ps == nil {
					break
				}
				exist, err := q.finalCheckSnapshot(ps.key)
				if err != nil {
					logger.Println("NewQueue finalCheckSnapshot", err)
					break
				}
				if !exist {
					snapshots = append(snapshots, ps)
				}
			}
			if len(snapshots) == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			err := q.writeFinals(snapshots)
			if err != nil {
				logger.Println("NewQueue writeFinals", err)
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
	opts.Prefix = []byte(cachePrefixTransactionCache)
	it := txn.NewIterator(opts)
	defer it.Close()

	var count uint64
	for it.Rewind(); it.Valid(); it.Next() {
		count = count + 1
	}
	offset := s.queue.snapshotQueueOffset()
	order := s.queue.snapshotQueueOrder()
	logger.Printf("QueueInfo %d %d %d %d %d\n", offset, order, s.queue.order, order-offset, s.queue.finalRing.Len())
	return count, order - offset, s.queue.cacheRing.Len(), nil
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
	start, limit := 0, 10000
	psc := make(chan []*PeerSnapshot)
	fuel := func(snapshots []*PeerSnapshot) {
		for !s.closing {
			select {
			case psc <- snapshots:
				return
			case <-time.After(3 * time.Second):
				logger.Println("QueuePollSnapshots hook TOO SLOW")
			}
		}
	}
	go func() {
		for !s.closing {
			snapshots, err := s.queue.batchRetrieveSnapshots(limit)
			if err != nil {
				logger.Println("QueuePollSnapshots batchRetrieveSnapshots", err)
			}
			fuel(snapshots)
		}
		close(psc)
	}()
	for !s.closing {
		for start = 0; start <= limit/10; start++ {
			ps, err := s.queue.PopCache()
			if err != nil {
				logger.Println("QueuePollSnapshots PopCache", err)
				break
			}
			if ps == nil {
				break
			}
			hook(ps.PeerId, ps.Snapshot)
		}
		select {
		case snapshots := <-psc:
			for _, ps := range snapshots {
				hook(ps.PeerId, ps.Snapshot)
			}
			if len(snapshots) < limit && start < 1 {
				time.Sleep(100 * time.Millisecond)
			}
		case <-time.After(1 * time.Second):
			logger.Println("QueuePollSnapshots batchRetrieveSnapshots TOO SLOW")
		}
	}
}

func (q *Queue) finalCheckSnapshot(key []byte) (bool, error) {
	txn := q.db.NewTransaction(false)
	defer txn.Discard()

	_, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (q *Queue) writeFinals(snapshots []*PeerSnapshot) error {
	wb := q.db.NewWriteBatch()
	defer wb.Cancel()
	for _, ps := range snapshots {
		key := cacheSnapshotQueueKey(q.order)
		val := common.MsgpackMarshalPanic(ps)
		err := wb.Set(key, val, 0)
		if err != nil {
			return err
		}
		err = wb.Set(ps.key, []byte{1}, 0)
		if err != nil {
			return err
		}
		q.order = q.order + 1
	}
	return wb.Flush()
}

func (q *Queue) batchRetrieveSnapshots(limit int) ([]*PeerSnapshot, error) {
	orders := make([]uint64, 0)
	snapshots := make([]*PeerSnapshot, 0)
	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(cachePrefixSnapshotQueue)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid() && len(snapshots) < limit; it.Next() {
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
	for _, ps := range snapshots {
		err := wb.Delete(ps.key)
		if err != nil {
			return snapshots, err
		}
	}
	return snapshots, wb.Flush()
}

func (q *Queue) snapshotQueueOffset() uint64 {
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
	return sequence
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
