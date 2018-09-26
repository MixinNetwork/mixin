package storage

import (
	"encoding/binary"
	"sync"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

type TopologicalSequence struct {
	sync.Mutex
	seq uint64
}

const snapshotsPrefixTopology = "TOPOLOGY" // local topological sorted snapshots, irreverlant to the consensus rule

func (s *BadgerStore) SnapshotsListTopologySince(topologyOffset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	snapshots := make([]*common.SnapshotWithTopologicalOrder, 0)

	err := s.snapshotsDB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		it.Seek(topologyKey(topologyOffset))
		for ; it.ValidForPrefix([]byte(snapshotsPrefixTopology)) && uint64(len(snapshots)) < count; it.Next() {
			item := it.Item()
			v, err := item.Value()
			if err != nil {
				return err
			}
			var s common.SnapshotWithTopologicalOrder
			err = msgpack.Unmarshal(v, &s)
			if err != nil {
				return err
			}
			s.TopologicalOrder = topologyOrder(item.Key())
			s.Hash = s.Transaction.Hash()
			s.Signatures = nil
			snapshots = append(snapshots, &s)
		}
		return nil
	})
	return snapshots, err
}

func saveSnapshotTopology(txn *badger.Txn, s *common.Snapshot, seq uint64) error {
	key := topologyKey(seq)
	val := common.MsgpackMarshalPanic(s)
	return txn.Set(key, val)
}

func (c *TopologicalSequence) Next() uint64 {
	c.Lock()
	defer c.Unlock()
	next := c.seq
	c.seq = c.seq + 1
	return next
}

func getTopologyCounter(db *badger.DB) *TopologicalSequence {
	c := &TopologicalSequence{}

	txn := db.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(topologyKey(^uint64(0)))
	if it.ValidForPrefix([]byte(snapshotsPrefixTopology)) {
		it.Next()
		item := it.Item()
		c.seq = topologyOrder(item.Key()) + 1
	}
	return c
}

func topologyKey(order uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, order)
	return append([]byte(snapshotsPrefixTopology), buf...)
}

func topologyOrder(key []byte) uint64 {
	order := key[len(snapshotsPrefixTopology):]
	return binary.BigEndian.Uint64(order)
}
