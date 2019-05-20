package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) QueueTransaction(tx *common.VersionedTransaction) (string, error) {
	err := tx.Validate(node.persistStore)
	if err != nil {
		return "", err
	}
	err = node.persistStore.CachePutTransaction(tx)
	if err != nil {
		return "", err
	}
	err = node.QueueAppendSnapshot(node.IdForNetwork, &common.Snapshot{
		NodeId:      node.IdForNetwork,
		Transaction: tx.PayloadHash(),
	}, false)
	return tx.PayloadHash().String(), err
}

func (node *Node) LoadCacheToQueue() error {
	return node.persistStore.CacheListTransactions(func(tx *common.VersionedTransaction) error {
		return node.QueueAppendSnapshot(node.IdForNetwork, &common.Snapshot{
			NodeId:      node.IdForNetwork,
			Transaction: tx.PayloadHash(),
		}, false)
	})
}

func (node *Node) ConsumeQueue() error {
	node.persistStore.QueuePollSnapshots(func(peerId crypto.Hash, snap *common.Snapshot) error {
		tx, err := node.persistStore.CacheGetTransaction(snap.Transaction)
		if err != nil {
			return err
		}
		if tx != nil {
			node.mempoolChan <- snap
			return nil
		}
		tx, err = node.persistStore.ReadTransaction(snap.Transaction)
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
		return node.QueueAppendSnapshot(peerId, snap, node.verifyFinalization(snap.Timestamp, snap.Signatures))
	})
	return nil
}
