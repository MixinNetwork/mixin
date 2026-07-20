package kernel

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
)

type TopologicalSequence struct {
	sync.Mutex
	seq   uint64
	point uint64
	sps   float64
	spt   float64

	filter map[crypto.Hash]bool
	check  uint64
	count  uint64
	tps    float64

	snapshotCounts map[crypto.Hash]uint64
}

func (node *Node) TopologicalOrder() uint64 {
	return node.TopoCounter.seq
}

func (node *Node) SPS() float64 {
	return node.TopoCounter.sps
}

func (node *Node) SPT() float64 {
	return node.TopoCounter.spt
}

func (node *Node) TPS() float64 {
	return node.TopoCounter.tps
}

type SnapshotWitness struct {
	Signature *crypto.Signature
	Timestamp uint64
}

func (node *Node) WitnessSnapshot(s *common.SnapshotWithTopologicalOrder) *SnapshotWitness {
	msg := crypto.Blake3Hash(s.VersionedMarshal())
	sig := node.Signer.PrivateSpendKey.Sign(msg)
	return &SnapshotWitness{
		Signature: &sig,
		Timestamp: clock.NowUnixNano(),
	}
}

func (node *Node) TopoWrite(s *common.Snapshot, signers []crypto.Hash) *common.SnapshotWithTopologicalOrder {
	logger.Debugf("node.TopoWrite(%v)\n", s)
	node.TopoCounter.Lock()
	defer node.TopoCounter.Unlock()

	if len(signers) != len(s.Signature.Keys()) {
		panic(fmt.Errorf("malformed snapshot signers %s %d %d", s.Hash, len(signers), len(s.Signature.Keys())))
	}
	if node.TopoCounter.seq%100000 == 7 {
		node.TopoCounter.filter = make(map[crypto.Hash]bool)
	}
	for _, tx := range s.Transactions {
		node.TopoCounter.snapshotCounts[tx] += 1
		if node.TopoCounter.filter[tx] {
			continue
		}
		node.TopoCounter.filter[tx] = true
		node.TopoCounter.count += 1
	}

	node.TopoCounter.seq += 1
	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         s,
		TopologicalOrder: node.TopoCounter.seq,
	}
	err := node.persistStore.WriteSnapshot(topo, signers)
	if err != nil {
		panic(err)
	}
	return topo
}

func (topo *TopologicalSequence) TopoStats(node *Node) {
	const durationSeconds = 60

	ticker := time.NewTicker(time.Duration(durationSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-node.done:
			return
		case <-ticker.C:
			topo.Lock()
			snapshots := topo.seq - topo.point
			transactions := topo.count - topo.check
			topo.sps = math.Round(float64(snapshots)/float64(durationSeconds)*100) / 100
			topo.point = topo.seq

			topo.tps = math.Round(float64(transactions)/float64(durationSeconds)*100) / 100
			topo.check = topo.count

			var snapshotCount uint64
			for _, count := range topo.snapshotCounts {
				snapshotCount += count
			}
			topo.spt = 0
			if len(topo.snapshotCounts) > 0 {
				topo.spt = math.Round(float64(snapshotCount)/float64(len(topo.snapshotCounts))*100) / 100
			}
			topo.snapshotCounts = make(map[crypto.Hash]uint64)
			topo.Unlock()
		}
	}
}

func (node *Node) getTopologyCounter(store storage.Store) *TopologicalSequence {
	s, _ := store.LastSnapshot()
	topo := &TopologicalSequence{
		seq:            s.TopologicalOrder,
		filter:         make(map[crypto.Hash]bool),
		snapshotCounts: make(map[crypto.Hash]uint64),
	}
	topo.point = topo.seq
	go topo.TopoStats(node)
	return topo
}
