package storage

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/allegro/bigcache"
	"github.com/dgraph-io/badger"
)

type Queue struct {
	cacheRing *RingBuffer
	finalRing *RingBuffer
	cache     *bigcache.BigCache
}

type PeerSnapshot struct {
	key      string
	PeerId   crypto.Hash
	Snapshot *common.Snapshot
}

func (ps *PeerSnapshot) cacheKey() string {
	hash := ps.Snapshot.PayloadHash()
	hash = hash.ForNetwork(ps.PeerId)
	for _, sig := range ps.Snapshot.Signatures {
		hash = crypto.NewHash(append(hash[:], sig[:]...))
	}
	return "BADGER:QUEUE:CACHE:" + hash.String()
}

func (ps *PeerSnapshot) finalKey() string {
	hash := ps.Snapshot.PayloadHash().ForNetwork(ps.PeerId)
	return "BADGER:QUEUE:FINAL:" + hash.String()
}

func NewQueue(cache *bigcache.BigCache) *Queue {
	return &Queue{
		cache:     cache,
		cacheRing: NewRingBuffer(1024 * 1024),
		finalRing: NewRingBuffer(1024 * 1024 * 16),
	}
}

func (q *Queue) Dispose() {
	q.finalRing.Dispose()
	q.cacheRing.Dispose()
}

func (q *Queue) PutFinal(ps *PeerSnapshot) error {
	ps.key = ps.finalKey()
	_, err := q.cache.Get(ps.key)
	if err != bigcache.ErrEntryNotFound {
		return err
	}

	err = q.cache.Set(ps.key, []byte{})
	if err != nil {
		return err
	}
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
	return ps, q.cache.Delete(ps.key)
}

func (q *Queue) PutCache(ps *PeerSnapshot) error {
	ps.key = ps.cacheKey()
	_, err := q.cache.Get(ps.key)
	if err != bigcache.ErrEntryNotFound {
		return err
	}

	err = q.cache.Set(ps.key, []byte{})
	if err != nil {
		return err
	}
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
	return ps, q.cache.Delete(ps.key)
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
