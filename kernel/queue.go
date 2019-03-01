package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) QueueTransaction(tx *common.SignedTransaction) (string, error) {
	err := tx.Validate(node.store)
	if err != nil {
		return "", err
	}
	err = node.store.CachePutTransaction(tx)
	if err != nil {
		return "", err
	}
	err = node.store.QueueAppendSnapshot(node.IdForNetwork, &common.Snapshot{
		NodeId:      node.IdForNetwork,
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

		if peerId == node.IdForNetwork {
			return nil
		}
		node.Peer.SendTransactionRequestMessage(peerId, snap.Transaction)
		return node.store.QueueAppendSnapshot(peerId, snap, node.verifyFinalization(snap.Signatures))
	})
	return nil
}
