package storage

import (
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

type Queue struct {
	mutex *sync.Mutex
	ring  *RingBuffer
	set   map[crypto.Hash]bool
}

type PeerSnapshot struct {
	PeerId   crypto.Hash
	Snapshot *common.Snapshot
}

func NewQueue() *Queue {
	return &Queue{
		mutex: new(sync.Mutex),
		set:   make(map[crypto.Hash]bool),
		ring:  NewRingBuffer(1024 * 1024),
	}
}

func (q *Queue) Dispose() {
	q.ring.Dispose()
}

func (q *Queue) Len() uint64 {
	return q.ring.Len()
}

func (q *Queue) Put(ps *PeerSnapshot) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	hash := ps.Snapshot.PayloadHash()
	if q.set[hash] {
		return nil
	}
	q.set[hash] = true

	for {
		put, err := q.ring.Offer(ps)
		if err != nil || put {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func (q *Queue) Pop() (*PeerSnapshot, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	item, err := q.ring.Poll(200 * time.Millisecond)
	if err != nil {
		return nil, err
	}
	ps := item.(*PeerSnapshot)
	delete(q.set, ps.Snapshot.PayloadHash())
	return ps, nil
}

func (s *BadgerStore) QueueInfo() (uint64, uint64, error) {
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
	return count, s.queue.Len(), nil
}

func (s *BadgerStore) QueueAppendSnapshot(peerId crypto.Hash, snap *common.Snapshot) error {
	return s.queue.Put(&PeerSnapshot{
		PeerId:   peerId,
		Snapshot: snap,
	})
}

func (s *BadgerStore) QueuePollSnapshots(hook func(peerId crypto.Hash, snap *common.Snapshot) error) {
	for !s.closing {
		ps, err := s.queue.Pop()
		if err != nil {
			continue
		}
		hook(ps.PeerId, ps.Snapshot)
	}
}
