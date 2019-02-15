package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/network"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/vmihailenco/msgpack"
)

func QueueTransaction(store storage.Store, tx *common.SignedTransaction) (string, error) {
	err := tx.Validate(store)
	if err != nil {
		return "", err
	}
	return tx.PayloadHash().String(), store.CacheAppendTransactionToQueue(tx)
}

func (node *Node) ConsumeQueue() error {
	var offset = uint64(0)
	for {
		err := node.store.CachePollTransactionsQueue(offset, func(k uint64, v []byte) error {
			var tx common.SignedTransaction
			err := msgpack.Unmarshal(v, &tx)
			if err != nil {
				return err
			}
			peer := network.NewPeer(node, node.IdForNetwork, "")
			err = node.FeedMempool(peer, &common.Snapshot{
				NodeId:            node.IdForNetwork,
				Transaction:       tx.PayloadHash(),
				SignedTransaction: &tx,
			})
			if err != nil {
				return err
			}
			offset = k
			return nil
		})
		if err != nil {
			panic(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}
