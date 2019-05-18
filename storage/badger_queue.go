package storage

import (
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

type Queue struct {
	mutex     *sync.Mutex
	cacheRing *RingBuffer
	finalRing *RingBuffer
	finalSet  map[crypto.Hash]bool
	cacheSet  map[crypto.Hash]bool
}

type PeerSnapshot struct {
	key      crypto.Hash
	PeerId   crypto.Hash
	Snapshot *common.Snapshot
}

func (ps *PeerSnapshot) cacheKey() crypto.Hash {
	hash := ps.Snapshot.PayloadHash()
	hash = hash.ForNetwork(ps.PeerId)
	for _, sig := range ps.Snapshot.Signatures {
		hash = crypto.NewHash(append(hash[:], sig[:]...))
	}
	return hash
}

func (ps *PeerSnapshot) finalKey() crypto.Hash {
	hash := ps.Snapshot.PayloadHash()
	return hash.ForNetwork(ps.PeerId)
}

func NewQueue() *Queue {
	return &Queue{
		mutex:     new(sync.Mutex),
		finalSet:  make(map[crypto.Hash]bool),
		cacheSet:  make(map[crypto.Hash]bool),
		cacheRing: NewRingBuffer(1024 * 1024),
		finalRing: NewRingBuffer(1024 * 1024 * 16),
	}
}

func (q *Queue) Dispose() {
	q.finalRing.Dispose()
	q.cacheRing.Dispose()
}

func (q *Queue) PutFinal(ps *PeerSnapshot) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.finalSet[ps.key] {
		return nil
	}
	q.finalSet[ps.key] = true

	for {
		put, err := q.finalRing.Offer(ps)
		if err != nil || put {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (q *Queue) PopFinal() (*PeerSnapshot, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	item, err := q.finalRing.Poll(false)
	if err != nil || item == nil {
		return nil, err
	}
	ps := item.(*PeerSnapshot)
	delete(q.finalSet, ps.key)
	return ps, nil
}

func (q *Queue) PutCache(ps *PeerSnapshot) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.cacheSet[ps.key] {
		return nil
	}
	q.cacheSet[ps.key] = true

	for {
		put, err := q.cacheRing.Offer(ps)
		if err != nil || put {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (q *Queue) PopCache() (*PeerSnapshot, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	item, err := q.cacheRing.Poll(false)
	if err != nil || item == nil {
		return nil, err
	}
	ps := item.(*PeerSnapshot)
	delete(q.cacheSet, ps.key)
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
	return count, s.queue.finalRing.Len(), s.queue.cacheRing.Len(), nil
}

func (s *BadgerStore) QueueAppendSnapshot(peerId crypto.Hash, snap *common.Snapshot, finalized bool) error {
	ps := &PeerSnapshot{
		PeerId:   peerId,
		Snapshot: snap,
	}
	if finalized {
		ps.key = ps.finalKey()
		return s.queue.PutFinal(ps)
	}
	ps.key = ps.cacheKey()
	return s.queue.PutCache(ps)
}

func (s *BadgerStore) QueuePollSnapshots(hook func(peerId crypto.Hash, snap *common.Snapshot) error) {
	for !s.closing {
		time.Sleep(1 * time.Millisecond)
		final, cache := 0, 0
		for i := 0; i < 10; i++ {
			ps, err := s.queue.PopFinal()
			if err != nil {
				break
			}
			if ps != nil {
				hook(ps.PeerId, ps.Snapshot)
				final++
			}
		}
		for i := 0; i < 2; i++ {
			ps, err := s.queue.PopCache()
			if err != nil {
				break
			}
			if ps != nil {
				hook(ps.PeerId, ps.Snapshot)
				cache++
			}
		}
		if cache < 1 && final < 1 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}
