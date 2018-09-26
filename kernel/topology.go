package kernel

import (
	"sync"

	"github.com/MixinNetwork/mixin/storage"
)

type TopologicalSequence struct {
	sync.Mutex
	seq uint64
}

func (c *TopologicalSequence) Next() uint64 {
	c.Lock()
	defer c.Unlock()
	next := c.seq
	c.seq = c.seq + 1
	return next
}

func getTopologyCounter(store storage.Store) *TopologicalSequence {
	return &TopologicalSequence{
		seq: store.SnapshotsTopologySequence(),
	}
}
