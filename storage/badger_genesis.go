package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger/v2"
)

func (s *BadgerStore) LoadGenesis(rounds []*common.Round, snapshots []*common.SnapshotWithTopologicalOrder, transactions []*common.VersionedTransaction) error {
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
	for i, snap := range snapshots {
		err := writeTransaction(txn, transactions[i])
		if err != nil {
			return err
		}
		err = writeSnapshot(txn, snap, transactions[i])
		if err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (s *BadgerStore) CheckGenesisLoad() (bool, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return checkGenesisLoad(txn), nil
}

func checkGenesisLoad(txn *badger.Txn) bool {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Rewind()
	return it.Valid()
}
