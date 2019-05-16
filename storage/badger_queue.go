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
	snapshots chan []*PeerSnapshot
}

type PeerSnapshot struct {
	Key      []byte
	PeerId   crypto.Hash
	Snapshot *common.Snapshot
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
		snapshots: make(chan []*PeerSnapshot, 32),
	}
	go func() {
		nodes := q.loadSnapshotNodesList()
		for id, _ := range nodes {
			go q.loopRetrieveSnapshotsForNode(id)
		}
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
				if id := ps.Snapshot.NodeId; !nodes[id] {
					nodes[id] = true
					err = q.writeNodeMeta(id)
					if err != nil {
						logger.Println("NewQueue writeNodeMeta", err)
						break
					}
					go q.loopRetrieveSnapshotsForNode(id)
				}
				snapshots = append(snapshots, ps)
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
	hash := ps.Snapshot.Hash.ForNetwork(ps.PeerId)
	ps.Key = hash[:]

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
	return item.(*PeerSnapshot), nil
}

func (q *Queue) PutCache(ps *PeerSnapshot) error {
	ps.Key = ps.cacheKey()
	data := q.cache.Get(nil, ps.Key)
	if len(data) > 0 {
		return nil
	}

	q.cache.Set(ps.Key, []byte{0})
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
	q.cache.Del(ps.Key)
	return ps, nil
}

func (s *BadgerStore) cacheTransactionsCount() uint64 {
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
	return count
}

func (s *BadgerStore) cacheSnapshotsCount() uint64 {
	txn := s.cacheDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Prefix = []byte(cachePrefixSnapshotNodeQueue)
	it := txn.NewIterator(opts)
	defer it.Close()

	var count uint64
	for it.Rewind(); it.Valid(); it.Next() {
		count = count + 1
	}
	return count
}

func (s *BadgerStore) QueueInfo() (uint64, uint64, uint64, error) {
	return s.cacheTransactionsCount(), s.cacheSnapshotsCount(), s.queue.cacheRing.Len(), nil
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
	start, limit := 0, 64
	for !s.closing {
		for start = 0; start < limit; start++ {
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
		case snapshots := <-s.queue.snapshots:
			for _, ps := range snapshots {
				hook(ps.PeerId, ps.Snapshot)
			}
			if len(snapshots) < 1 && start < 1 {
				time.Sleep(100 * time.Millisecond)
			}
		case <-time.After(1 * time.Second):
			logger.Println("QueuePollSnapshots batchRetrieveSnapshots TOO SLOW")
		}
	}
}

func (q *Queue) loadSnapshotNodesList() map[crypto.Hash]bool {
	txn := q.db.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(cachePrefixSnapshotNodeMeta)
	it := txn.NewIterator(opts)
	defer it.Close()

	nodes := make(map[crypto.Hash]bool)
	for it.Rewind(); it.Valid(); it.Next() {
		key := it.Item().Key()
		var hash crypto.Hash
		copy(hash[:], key[len(cachePrefixSnapshotNodeMeta):])
		nodes[hash] = true
	}
	return nodes
}

func (q *Queue) loopRetrieveSnapshotsForNode(nodeId crypto.Hash) {
	fuel := func(snapshots []*PeerSnapshot) {
		for {
			select {
			case q.snapshots <- snapshots:
				return
			case <-time.After(5 * time.Second):
				logger.Println("QueuePollSnapshots hook TOO SLOW")
			}
		}
	}
	for {
		snapshots, err := q.batchRetrieveSnapshotsForNode(nodeId, 1024)
		if err != nil {
			logger.Println("QueuePollSnapshots batchRetrieveSnapshots", err)
		}
		if len(snapshots) > 0 {
			fuel(snapshots)
		} else {
			time.Sleep(300 * time.Millisecond)
		}
	}
}

func (q *Queue) writeNodeMeta(id crypto.Hash) error {
	txn := q.db.NewTransaction(true)
	defer txn.Discard()

	key := append([]byte(cachePrefixSnapshotNodeMeta), id[:]...)
	err := txn.Set(key, []byte{1})
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (q *Queue) writeFinals(snapshots []*PeerSnapshot) error {
	wb := q.db.NewWriteBatch()
	defer wb.Cancel()
	for _, ps := range snapshots {
		key := cacheSnapshotNodeQueueKey(ps)
		val := common.MsgpackMarshalPanic(ps)
		err := wb.Set(key, val, 0)
		if err != nil {
			return err
		}
	}
	return wb.Flush()
}

func (q *Queue) batchRetrieveSnapshotsForNode(nodeId crypto.Hash, limit int) ([]*PeerSnapshot, error) {
	keys := make([][]byte, 0)
	snapshots := make([]*PeerSnapshot, 0)
	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(cachePrefixSnapshotNodeQueue)
		opts.Prefix = append(opts.Prefix, nodeId[:]...)
		it := txn.NewIterator(opts)
		defer it.Close()

		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, 0)
		it.Seek(append(opts.Prefix, buf...))
		for ; it.ValidForPrefix(opts.Prefix) && len(snapshots) < limit; it.Next() {
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
			keys = append(keys, item.KeyCopy(nil))
		}
		return nil
	})
	if err != nil {
		return snapshots, err
	}

	wb := q.db.NewWriteBatch()
	defer wb.Cancel()
	for _, key := range keys {
		err := wb.Delete(key)
		if err != nil {
			return snapshots, err
		}
	}
	return snapshots, wb.Flush()
}

func cacheSnapshotNodeQueueKey(ps *PeerSnapshot) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, ps.Snapshot.RoundNumber)
	key := append(ps.Snapshot.NodeId[:], buf...)
	key = append([]byte(cachePrefixSnapshotNodeQueue), key...)
	return append(key, ps.Key...)
}
