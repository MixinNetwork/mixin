package p2p

import (
	"sync"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestConcurrentConsumerReplacementRetainsCurrentPeer(t *testing.T) {
	id := crypto.Blake3Hash([]byte("reconnecting consumer"))
	original := NewPeer(nil, id, "original", false)
	neighbors := &neighborMap{m: map[crypto.Hash]*Peer{id: original}}
	start := make(chan struct{})
	previous := make(chan *Peer, 32)
	var wg sync.WaitGroup
	for range cap(previous) {
		wg.Go(func() {
			peer := NewPeer(nil, id, "replacement", false)
			<-start
			previous <- neighbors.Swap(id, peer)
		})
	}
	close(start)
	wg.Wait()
	close(previous)

	current := neighbors.Get(id)
	require.NotSame(t, original, current)
	retired := make(map[*Peer]bool)
	for old := range previous {
		require.NotNil(t, old)
		require.False(t, retired[old], "each connection must be replaced once")
		retired[old] = true
		// A replaced connection's deferred cleanup may run after another
		// connection has already become the current neighbor.
		neighbors.DeleteIf(id, old)
		require.Same(t, current, neighbors.Get(id))
	}
	require.True(t, retired[original])
	require.False(t, retired[current])
	neighbors.DeleteIf(id, current)
	require.Nil(t, neighbors.Get(id))
}
