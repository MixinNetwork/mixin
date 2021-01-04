package kernel

import (
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/storage"
)

type TopologicalSequence struct {
	sync.Mutex
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

type SnapshotWitness struct {
	Signature *crypto.Signature `json:"signature"`
	Timestamp uint64            `json:"timestamp"`
}

func (node *Node) WitnessSnapshot(s *common.SnapshotWithTopologicalOrder) *SnapshotWitness {
	msg := crypto.NewHash(common.MsgpackMarshalPanic(s))
	sig := node.Signer.PrivateSpendKey.Sign(msg[:])
	return &SnapshotWitness{
		Signature: &sig,
		Timestamp: uint64(clock.Now().UnixNano()),
	}
}

func (node *Node) TopoWrite(s *common.Snapshot) *common.SnapshotWithTopologicalOrder {
	node.TopoCounter.Lock()
	defer node.TopoCounter.Unlock()

	node.TopoCounter.seq += 1
	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: node.TopoCounter.seq,
	}
	err := node.persistStore.WriteSnapshot(topo)
	if err != nil {
		panic(err)
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
