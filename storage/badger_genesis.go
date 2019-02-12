package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

func (s *BadgerStore) LoadGenesis(snapshots []*common.SnapshotWithTopologicalOrder) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	if checkGenesisLoad(txn) {
		return nil
	}

	filter := make(map[crypto.Hash]bool)
	for _, snap := range snapshots {
		if !filter[snap.NodeId] {
			filter[snap.NodeId] = true
			err := startNewRound(txn, snap.NodeId, snap.RoundNumber, snap.Timestamp, snap.References)
			if err != nil {
				return err
			}
		}
		err := writeSnapshot(txn, snap)
		if err != nil {
			return err
		}
	}

	return txn.Commit()
}

func checkGenesisLoad(txn *badger.Txn) bool {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Rewind()
	return it.Valid()
}
