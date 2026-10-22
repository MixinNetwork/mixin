package kernel

import (
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/p2p"
)

func TestGraphBroadcastStopsOnShutdown(t *testing.T) {
	node := &Node{
		Peer: p2p.NewPeer(nil, crypto.Hash{}, "", false),
		done: make(chan struct{}),
	}
	stopped := make(chan struct{})
	go func() {
		node.sendGraphToConsensusNodesAndPeers()
		close(stopped)
	}()
	close(node.done)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("graph broadcast did not stop after node shutdown")
	}
}
