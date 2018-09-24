package kernel

import (
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
)

func Loop(store storage.Store, addr string, dir string) error {
	node, err := setupNode(store, addr, dir)
	if err != nil {
		return err
	}
	roundHash, err := node.RoundHash()
	if err != nil {
		return err
	}
	logger.Printf("Round #%d (%s)\n", node.RoundNumber, roundHash)
	panicGo(node.ConsumeMempool)
	panicGo(node.ConsumeQueue)
	return node.ListenPeers()
}
