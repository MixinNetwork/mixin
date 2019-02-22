package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

func QueueTransaction(store storage.Store, tx *common.SignedTransaction) (string, error) {
	err := tx.Validate(store)
	if err != nil {
		return "", err
	}
	err = store.CachePutTransaction(tx)
	if err != nil {
		return "", err
	}
	err = store.QueueAppendSnapshot(NodeIdForNetwork(), &common.Snapshot{
		NodeId:      NodeIdForNetwork(),
		Transaction: tx.PayloadHash(),
	}, false)
	return tx.PayloadHash().String(), err
}

func (node *Node) LoadCacheToQueue() error {
	return node.store.CacheListTransactions(func(tx *common.SignedTransaction) error {
		return node.store.QueueAppendSnapshot(node.IdForNetwork, &common.Snapshot{
			NodeId:      node.IdForNetwork,
			Transaction: tx.PayloadHash(),
		}, false)
	})
}

func (node *Node) ConsumeQueue() error {
	node.store.QueuePollSnapshots(func(peerId crypto.Hash, snap *common.Snapshot) error {
		tx, err := node.store.CacheGetTransaction(snap.Transaction)
		if err != nil {
			return err
		}
		if tx != nil {
			node.mempoolChan <- snap
			return nil
		}
		tx, err = node.store.ReadTransaction(snap.Transaction)
		if err != nil {
			return err
		}
		if tx != nil {
			node.mempoolChan <- snap
			return nil
		}

		if peerId == node.IdForNetwork || snap.NodeId == node.IdForNetwork {
			return nil
		}
		node.Peer.SendTransactionRequestMessage(peerId, snap.Transaction)
		return node.store.QueueAppendSnapshot(peerId, snap, node.verifyFinalization(snap.Signatures))
	})
	return nil
}
