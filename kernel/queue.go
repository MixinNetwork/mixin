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
	err = store.QueueAppendSnapshot(crypto.Hash{}, &common.Snapshot{
		Transaction: tx.PayloadHash(),
	})
	return tx.PayloadHash().String(), err
}

func (node *Node) LoadCacheToQueue() error {
	return node.store.CacheListTransactions(func(tx *common.SignedTransaction) error {
		return node.store.QueueAppendSnapshot(crypto.Hash{}, &common.Snapshot{
			Transaction: tx.PayloadHash(),
		})
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

		if peerId == node.IdForNetwork || !peerId.HasValue() {
			return nil
		}
		if !snap.NodeId.HasValue() {
			return nil
		}
		node.Peer.SendTransactionRequestMessage(peerId, snap.Transaction)
		return node.store.QueueAppendSnapshot(peerId, snap)
	})
	return nil
}
