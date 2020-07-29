package kernel

import (
	"sync/atomic"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
)

type TopologicalSequence struct {
	seq   uint64
	point uint64
	sps   float64
}

func (node *Node) TopologicalOrder() uint64 {
	return node.TopoCounter.seq
}

func (node *Node) SPS() float64 {
	return node.TopoCounter.sps
}

func (node *Node) TopoWrite(s *common.Snapshot) *common.SnapshotWithTopologicalOrder {
	next := atomic.AddUint64(&node.TopoCounter.seq, 1)
	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: next,
	}
	for {
		err := node.persistStore.WriteSnapshot(topo)
		if err != nil {
			logger.Println("TopoWrite", s.Transaction, err)
			continue
		}
		break
	}
	return topo
}

func (topo *TopologicalSequence) TopoStats() {
	durationSeconds := 60
	for {
		time.Sleep(time.Duration(durationSeconds) * time.Second)
		topo.sps = float64(topo.seq-topo.point) / float64(durationSeconds)
		topo.point = topo.seq
	}
}

func getTopologyCounter(store storage.Store) *TopologicalSequence {
	topo := &TopologicalSequence{
		seq: store.TopologySequence(),
	}
	topo.point = topo.seq
	go topo.TopoStats()
	return topo
}
