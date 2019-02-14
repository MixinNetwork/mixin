package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger"
)

func (s *BadgerStore) LoadGenesis(rounds []*common.Round, snapshots []*common.SnapshotWithTopologicalOrder) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	if checkGenesisLoad(txn) {
		return nil
	}

	for _, r := range rounds {
		err := writeRound(txn, r.Hash, r)
		if err != nil {
			return err
		}
	}
	for _, snap := range snapshots {
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
