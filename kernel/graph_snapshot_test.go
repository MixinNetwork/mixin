package kernel

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal"
	"github.com/MixinNetwork/mixin/p2p"
	"github.com/stretchr/testify/require"
)

func TestBuildGraphSnapshotLoading(t *testing.T) {
	wasMocked := internal.MockRunAggregators()
	internal.ToggleMockRunAggregators(true)
	t.Cleanup(func() { internal.ToggleMockRunAggregators(wasMocked) })

	node := setupTestNode(require.New(t), t.TempDir())
	t.Cleanup(func() {
		node.cacheStore.Close()
		require.NoError(t, node.persistStore.Close())
	})

	points := node.BuildGraph()
	require.Len(t, points, len(node.genesisNodes))
	for _, point := range points {
		chain := node.getChain(point.NodeId)
		require.Equal(t, chain.State.FinalRound.Hash, point.Hash)
		require.Equal(t, chain.State.FinalRound.Number, point.Number)
		require.Equal(t, map[string]int{"index": 0, "count": 0}, point.Pool)
	}
}

func TestBuildGraphSnapshotIsolation(t *testing.T) {
	node, chain := newGraphSnapshotTestChain()
	require.Empty(t, node.BuildGraph())

	final := graphSnapshotTestRound(chain.ChainId, 0)
	cache := &CacheRound{NodeId: chain.ChainId, Number: 1}
	chain.State = &ChainState{RoundHistory: []*FinalRound{final}}
	chain.assignNewGraphRound(final, cache)
	initial := node.BuildGraph()
	require.Len(t, initial, 1)
	require.Equal(t, &p2p.SyncPoint{
		NodeId: chain.ChainId,
		Hash:   final.Hash,
		Number: 0,
		Pool:   map[string]int{"index": 0, "count": 0},
	}, initial[0])

	// Node acceptance advances the pool before assigning the same final round.
	chain.StepForward()
	chain.assignNewGraphRound(final, cache)
	accepted := node.BuildGraph()
	require.Len(t, accepted, 1)
	require.Equal(t, uint64(0), accepted[0].Number)
	require.Equal(t, map[string]int{"index": 1, "count": 1}, accepted[0].Pool)
	require.Equal(t, map[string]int{"index": 0, "count": 0}, initial[0].Pool)

	final = graphSnapshotTestRound(chain.ChainId, 1)
	chain.assignNewGraphRound(final, &CacheRound{NodeId: chain.ChainId, Number: 2})
	expected := &p2p.SyncPoint{
		NodeId: chain.ChainId,
		Hash:   final.Hash,
		Number: 1,
		Pool:   map[string]int{"index": 2, "count": 2},
	}
	points := node.BuildGraph()
	require.Equal(t, []*p2p.SyncPoint{expected}, points)
	require.Equal(t, uint64(0), accepted[0].Number)
	require.Equal(t, map[string]int{"index": 1, "count": 1}, accepted[0].Pool)

	// Neither a caller's returned map nor the mutable source state may change
	// the published view, including while the next update is being prepared.
	points[0].Hash = crypto.Hash{}
	points[0].Number = 99
	points[0].Pool.(map[string]int)["index"] = 99
	points[0].Pool.(map[string]int)["count"] = 99
	final.Hash = crypto.Hash{}
	final.Number = 99
	chain.FinalIndex = 99
	chain.FinalCount = 99
	require.Equal(t, []*p2p.SyncPoint{expected}, node.BuildGraph())
}

func TestBuildGraphSnapshotConcurrentUpdates(t *testing.T) {
	node, chain := newGraphSnapshotTestChain()
	final := graphSnapshotTestRound(chain.ChainId, 0)
	chain.State = &ChainState{RoundHistory: []*FinalRound{final}}
	chain.assignNewGraphRound(final, &CacheRound{NodeId: chain.ChainId, Number: 1})

	const readers = 4
	const rounds = 10000
	start, done := make(chan struct{}), make(chan struct{})
	errors := make(chan error, readers)
	var wg sync.WaitGroup
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for {
				points := node.BuildGraph()
				if len(points) != 1 {
					errors <- fmt.Errorf("expected one chain, got %d", len(points))
					return
				}
				point := points[0]
				pool, ok := point.Pool.(map[string]int)
				if !ok || point.NodeId != chain.ChainId || binary.BigEndian.Uint64(point.Hash[:8]) != point.Number ||
					pool["count"] != int(point.Number) || pool["index"] != int(point.Number)%FinalPoolSlotsLimit {
					errors <- fmt.Errorf("inconsistent graph snapshot: %+v", point)
					return
				}
				select {
				case <-done:
					return
				default:
				}
			}
		}()
	}
	close(start)
	for number := uint64(1); number <= rounds; number++ {
		chain.assignNewGraphRound(graphSnapshotTestRound(chain.ChainId, number), &CacheRound{
			NodeId: chain.ChainId,
			Number: number + 1,
		})
		if number%64 == 0 {
			runtime.Gosched()
		}
	}
	close(done)
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	points := node.BuildGraph()
	require.Len(t, points, 1)
	require.Equal(t, uint64(rounds), points[0].Number)
	require.Equal(t, rounds, points[0].Pool.(map[string]int)["count"])
}

func newGraphSnapshotTestChain() (*Node, *Chain) {
	node := &Node{chains: &chainsMap{m: make(map[crypto.Hash]*Chain)}}
	chain := &Chain{node: node, ChainId: crypto.Blake3Hash([]byte("graph snapshot chain"))}
	node.chains.m[chain.ChainId] = chain
	return node, chain
}

func graphSnapshotTestRound(id crypto.Hash, number uint64) *FinalRound {
	final := &FinalRound{NodeId: id, Number: number, Start: number, End: number}
	binary.BigEndian.PutUint64(final.Hash[:8], number)
	return final
}
