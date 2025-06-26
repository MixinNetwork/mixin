package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/kernel/internal/clock"
)

func (node *Node) Loop() error {
	err := node.addRelayersFromConfig()
	if err != nil {
		return err
	}

	go node.listenConsumers()
	go node.sendGraphToConsensusNodesAndPeers()
	go node.loopCacheQueue()
	go node.MintLoop()
	node.ElectionLoop()
	return nil
}

func (node *Node) Teardown() {
	close(node.done)
	<-node.cqc
	<-node.mlc
	<-node.elc
	node.chains.RLock()
	for _, c := range node.chains.m {
		c.Teardown()
	}
	node.chains.RUnlock()
	node.Peer.Teardown()
	node.persistStore.Close()
	node.cacheStore.Clear()
}

func TestMockReset() {
	clock.Reset()
}

func TestMockDiff(at time.Duration) {
	clock.MockDiff(at)
}
