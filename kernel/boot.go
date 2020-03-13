package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/kernel/internal/clock"
)

func (node *Node) Loop() error {
	panicGo(node.ListenNeighbors)
	panicGo(node.CosiLoop)
	panicGo(node.MintLoop)
	panicGo(node.ElectionLoop)
	return node.ConsumeQueue()
}

func TestMockDiff(at time.Duration) {
	clock.MockDiff(at)
}
