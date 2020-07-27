package kernel

import (
	"sync"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/storage"
)

type TopologicalSequence struct {
	sync.Mutex
	seq uint64
}

func (node *Node) TopologicalOrder() uint64 {
	return node.TopoCounter.seq
}

func (node *Node) TopoWrite(s *common.Snapshot) *common.SnapshotWithTopologicalOrder {
	node.TopoCounter.Lock()
	defer node.TopoCounter.Unlock()

	next := node.TopoCounter.seq
	node.TopoCounter.seq += 1

	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: next,
	}
	err := node.persistStore.WriteSnapshot(topo)
	if err != nil {
		panic(err)
	}
	return topo
}

func getTopologyCounter(store storage.Store) *TopologicalSequence {
	return &TopologicalSequence{
		seq: store.TopologySequence(),
	}
}
