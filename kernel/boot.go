package kernel

import (
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

var globalNode *Node

func Loop(store storage.Store, addr string, dir string) error {
	node, err := setupNode(store, addr, dir)
	if err != nil {
		return err
	}
	globalNode = node
	panicGo(node.ListenPeers)
	panicGo(node.ConsumeMempool)
	node.SyncFinalGraphToAllPeers()
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
