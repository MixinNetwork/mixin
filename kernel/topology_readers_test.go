package kernel

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTopologyStatsDuringUpdates(t *testing.T) {
	node := &Node{TopoCounter: &TopologicalSequence{}}
	const updates = 1000
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 1; i <= updates; i++ {
			node.TopoCounter.Lock()
			node.TopoCounter.seq = uint64(i)
			node.TopoCounter.sps = float64(i) * 2
			node.TopoCounter.spt = float64(i) * 3
			node.TopoCounter.tps = float64(i) * 4
			node.TopoCounter.Unlock()
		}
	})
	for range updates {
		stats := node.TopologyStats()
		require.LessOrEqual(t, stats.TopologicalOrder, uint64(updates))
		require.Equal(t, float64(stats.TopologicalOrder)*2, stats.SPS)
		require.Equal(t, float64(stats.TopologicalOrder)*3, stats.SPT)
		require.Equal(t, float64(stats.TopologicalOrder)*4, stats.TPS)
	}
	wg.Wait()
	require.Equal(t, TopologyStats{
		TopologicalOrder: updates,
		SPS:              updates * 2,
		SPT:              updates * 3,
		TPS:              updates * 4,
	}, node.TopologyStats())
}
