package kernel

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/MixinNetwork/mixin/kernel/internal/clock"
)

func (node *Node) Loop() error {
	rand.Seed(time.Now().UnixNano())
	err := node.PingNeighborsFromConfig()
	if err != nil {
		return err
	}
	go func() {
		err := node.ListenNeighbors()
		if err != nil {
			panic(fmt.Errorf("ListenNeighbors %s", err.Error()))
		}
	}()
	go node.LoopCacheQueue()
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
	node.cacheStore.Reset()
}

func TestMockReset() {
	clock.Reset()
}

func TestMockDiff(at time.Duration) {
	clock.MockDiff(at)
}
