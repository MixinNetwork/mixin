package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

type PeerSnapshot struct {
	PeerId   crypto.Hash
	Snapshot *common.Snapshot
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
	return count, s.ring.Len(), nil
}

func (s *BadgerStore) QueueAppendSnapshot(peerId crypto.Hash, snap *common.Snapshot) error {
	s.ring.Put(&PeerSnapshot{
		PeerId:   peerId,
		Snapshot: snap,
	})
	return nil
}

func (s *BadgerStore) QueuePollSnapshots(hook func(peerId crypto.Hash, snap *common.Snapshot) error) {
	for !s.closing {
		item, err := s.ring.Poll(0)
		if err != nil {
			continue
		}
		ps := item.(*PeerSnapshot)
		hook(ps.PeerId, ps.Snapshot)
	}
}
