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
}

type PeerSnapshot struct {
	PeerId   crypto.Hash
	Snapshot *common.Snapshot
}

func NewQueue() *Queue {
	return &Queue{
		mutex:     new(sync.Mutex),
		finalSet:  make(map[crypto.Hash]bool),
		cacheRing: NewRingBuffer(1024 * 1024),
		finalRing: NewRingBuffer(1024 * 1024),
	}
}

func (q *Queue) Dispose() {
	q.finalRing.Dispose()
	q.cacheRing.Dispose()
}

func (q *Queue) PutFinal(ps *PeerSnapshot) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	hash := ps.Snapshot.PayloadHash()
	if q.finalSet[hash] {
		return nil
	}
	q.finalSet[hash] = true

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
	hash := ps.Snapshot.PayloadHash()
	delete(q.finalSet, hash)
	return ps, nil
}

func (q *Queue) PutCache(ps *PeerSnapshot) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

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
		return s.queue.PutFinal(ps)
	}
	return s.queue.PutCache(ps)
}

func (s *BadgerStore) QueuePollSnapshots(hook func(peerId crypto.Hash, snap *common.Snapshot) error) {
	for !s.closing {
		time.Sleep(1 * time.Millisecond)
		ps, err := s.queue.PopFinal()
		if err != nil {
			continue
		}
		if ps == nil {
			ps, err = s.queue.PopCache()
			if err != nil {
				continue
			}
		}
		if ps == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		hook(ps.PeerId, ps.Snapshot)
	}
}
