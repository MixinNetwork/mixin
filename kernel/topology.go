package kernel

import (
	"fmt"
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

	filter map[crypto.Hash]bool
	check  uint64
	count  uint64
	tps    float64
}

func (node *Node) TopologicalOrder() uint64 {
	return node.TopoCounter.seq
}

func (node *Node) SPS() float64 {
	return node.TopoCounter.sps
}

func (node *Node) TPS() float64 {
	return node.TopoCounter.tps
}

type SnapshotWitness struct {
	Signature *crypto.Signature
	Timestamp uint64
}

func (node *Node) WitnessSnapshot(s *common.SnapshotWithTopologicalOrder) *SnapshotWitness {
	msg := crypto.NewHash(common.MsgpackMarshalPanic(s))
	sig := node.Signer.PrivateSpendKey.Sign(msg[:])
	return &SnapshotWitness{
		Signature: &sig,
		Timestamp: uint64(clock.Now().UnixNano()),
	}
}

func (node *Node) TopoWrite(s *common.Snapshot, signers []crypto.Hash) *common.SnapshotWithTopologicalOrder {
	node.TopoCounter.Lock()
	defer node.TopoCounter.Unlock()

	if s.Version >= common.SnapshotVersion && len(signers) != len(s.Signature.Keys()) {
		panic(fmt.Errorf("malformed snapshot signers %s %d %d", s.Hash, len(signers), len(s.Signature.Keys())))
	}

	if node.TopoCounter.seq%100000 == 7 {
		node.TopoCounter.filter = make(map[crypto.Hash]bool)
	}
	if !node.TopoCounter.filter[s.Transaction] {
		node.TopoCounter.filter[s.Transaction] = true
		node.TopoCounter.count += 1
	}

	node.TopoCounter.seq += 1
	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: node.TopoCounter.seq,
	}
	err := node.persistStore.WriteSnapshot(topo, signers)
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

		topo.tps = float64(topo.count-topo.check) / float64(durationSeconds)
		topo.check = topo.count
	}
}

func getTopologyCounter(store storage.Store) *TopologicalSequence {
	topo := &TopologicalSequence{
		seq:    store.TopologySequence(),
		filter: make(map[crypto.Hash]bool),
	}
	topo.point = topo.seq
	go topo.TopoStats()
	return topo
}
