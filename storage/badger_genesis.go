package storage

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger/v3"
)

func (s *BadgerStore) LoadGenesis(rounds []*common.Round, snapshots []*common.SnapshotWithTopologicalOrder, transactions []*common.VersionedTransaction) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	loaded, err := checkGenesisLoad(txn, snapshots)
	if loaded || err != nil {
		return err
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
		err = writeSnapshotWork(txn, snap, nil)
		if err != nil {
			return err
		}
	}

	return txn.Commit()
}

func (s *BadgerStore) CheckGenesisLoad(snapshots []*common.SnapshotWithTopologicalOrder) (bool, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return checkGenesisLoad(txn, snapshots)
}

func checkGenesisLoad(txn *badger.Txn, snapshots []*common.SnapshotWithTopologicalOrder) (bool, error) {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	loaded, index := false, 0
	prefix := []byte(graphPrefixTopology)
	it.Seek(graphTopologyKey(0))
	for ; it.ValidForPrefix(prefix) && index < len(snapshots); it.Next() {
		loaded = true
		item := it.Item()
		v, err := item.ValueCopy(nil)
		if err != nil {
			return loaded, err
		}
		item, err = txn.Get(v)
		if err != nil {
			return loaded, err
		}
		v, err = item.ValueCopy(nil)
		if err != nil {
			return loaded, err
		}
		var snap common.SnapshotWithTopologicalOrder
		err = common.DecompressMsgpackUnmarshal(v, &snap)
		if err != nil {
			return loaded, err
		}
		hash := snap.PayloadHash()
		if hash != snapshots[index].Hash {
			return loaded, fmt.Errorf("malformed genesis snapshot %s %s", snapshots[index].Hash, hash)
		}
		index = index + 1
	}

	return loaded, nil
}
