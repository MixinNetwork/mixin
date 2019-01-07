package kernel

import (
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

var globalNode *Node

func Loop(store storage.Store, addr string, dir string) error {
	node, err := SetupNode(store, addr, dir)
	if err != nil {
		return err
	}
	globalNode = node
	panicGo(node.ListenNeighbors)
	panicGo(node.ConsumeMempool)
	return node.ConsumeQueue()
}

func NetworkId() crypto.Hash {
	if globalNode == nil {
		return crypto.Hash{}
	}
	return globalNode.networkId
}

func NodeIdForNetwork() crypto.Hash {
	if globalNode == nil {
		return crypto.Hash{}
	}
	return globalNode.IdForNetwork
}

func TopologicalOrder() uint64 {
	if globalNode == nil {
		return 0
	}
	return globalNode.TopoCounter.seq
}
