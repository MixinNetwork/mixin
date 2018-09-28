package kernel

import (
	"github.com/MixinNetwork/mixin/storage"
)

func Loop(store storage.Store, addr string, dir string) error {
	node, err := setupNode(store, addr, dir)
	if err != nil {
		return err
	}
	panicGo(node.ListenPeers)
	panicGo(node.ConsumeMempool)
	node.SyncFinalGraphToAllPeers()
	return node.ConsumeQueue()
}
