package storage

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ReadSnapshot(hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readSnapshotWithTopo(txn, hash)
}

func readSnapshotWithTopo(txn *badger.Txn, hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error) {
	item, err := txn.Get(graphSnapTopologyKey(hash))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	topo, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}

	item, err = txn.Get(topo)
	if err != nil {
		return nil, err
	}
	key, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}

	item, err = txn.Get(key)
	if err != nil {
		return nil, err
	}
	v, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	snap, err := common.UnmarshalVersionedSnapshot(v)
	if err != nil {
		return nil, err
	}
	snap.Hash = hash
	snap.TopologicalOrder = graphTopologyOrder(topo)
	return snap, nil
}

func (s *BadgerStore) ReadSnapshotWithTransactionsSinceTopology(topologyOffset, count uint64) ([]*common.SnapshotWithTopologicalOrder, [][]*common.VersionedTransaction, error) {
	if count > 500 {
		return nil, nil, fmt.Errorf("count %d too large, the maximum is 500", count)
	}
	snapshots, err := s.ReadSnapshotsSinceTopology(topologyOffset, count)
	if err != nil {
		return nil, nil, err
	}

	transactions := make([][]*common.VersionedTransaction, len(snapshots))
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	for i, s := range snapshots {
		for _, h := range s.Transactions {
			tx, err := readTransaction(txn, h)
			if err != nil {
				return nil, nil, err
			}
			transactions[i] = append(transactions[i], tx)
		}
	}
	return snapshots, transactions, nil
}

func (s *BadgerStore) ReadSnapshotsSinceTopology(topologyOffset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readSnapshotsSinceTopology(txn, topologyOffset, count)
}

func readSnapshotsSinceTopology(txn *badger.Txn, topologyOffset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	snapshots := make([]*common.SnapshotWithTopologicalOrder, 0)
	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(graphPrefixTopology)
	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(graphTopologyKey(topologyOffset))
	for ; it.Valid() && uint64(len(snapshots)) < count; it.Next() {
		item := it.Item()
		v, err := item.ValueCopy(nil)
		if err != nil {
			return snapshots, err
		}
		topology := graphTopologyOrder(item.KeyCopy(nil))
		item, err = txn.Get(v)
		if err != nil {
			return snapshots, err
		}
		v, err = item.ValueCopy(nil)
		if err != nil {
			return snapshots, err
		}
		snap, err := common.UnmarshalVersionedSnapshot(v)
		if err != nil {
			return snapshots, err
		}
		snap.Hash = snap.PayloadHash()
		snap.TopologicalOrder = topology
		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

func readLastTopology(txn *badger.Txn) uint64 {
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(graphTopologyKey(^uint64(0)))
	if it.ValidForPrefix([]byte(graphPrefixTopology)) {
		key := it.Item().KeyCopy(nil)
		return graphTopologyOrder(key)
	}
	return 0
}

func (s *BadgerStore) LastSnapshot() (*common.SnapshotWithTopologicalOrder, []*common.VersionedTransaction) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	topo := readLastTopology(txn)
	snaps, err := readSnapshotsSinceTopology(txn, topo, 10)
	if err != nil {
		panic(err)
	}
	if len(snaps) != 1 {
		panic(topo)
	}
	var txs []*common.VersionedTransaction
	for _, h := range snaps[0].Transactions {
		tx, err := readTransaction(txn, h)
		if err != nil {
			panic(err)
		}
		txs = append(txs, tx)
	}
	return snaps[0], txs
}

func writeTopology(txn *badger.Txn, snap *common.SnapshotWithTopologicalOrder) error {
	key := graphTopologyKey(snap.TopologicalOrder)
	val := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.PayloadHash())
	_, err := txn.Get(key)
	if err != badger.ErrKeyNotFound {
		panic(err)
	}
	err = txn.Set(key, val[:])
	if err != nil {
		return err
	}

	return txn.Set(graphSnapTopologyKey(snap.PayloadHash()), key)
}

func graphSnapTopologyKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixSnapTopology), hash[:]...)
}

func graphTopologyKey(order uint64) []byte {
	key := []byte(graphPrefixTopology)
	return binary.BigEndian.AppendUint64(key, order)
}

func graphTopologyOrder(key []byte) uint64 {
	order := key[len(graphPrefixTopology):]
	return binary.BigEndian.Uint64(order)
}
