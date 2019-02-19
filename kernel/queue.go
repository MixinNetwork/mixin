package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
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

func (node *Node) ConsumeQueue() error {
	filter := make(map[crypto.Hash]time.Time)
	node.store.QueuePollSnapshots(func(peerId crypto.Hash, snap *common.Snapshot) error {
		tx, err := node.store.CacheGetTransaction(snap.Transaction)
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
		hash := snap.Transaction.ForNetwork(peerId)
		if filter[hash].Add(time.Duration(config.SnapshotRoundGap * 2)).Before(time.Now()) {
			node.Peer.SendTransactionRequestMessage(peerId, snap.Transaction)
			filter[hash] = time.Now()
		}
		return node.store.QueueAppendSnapshot(peerId, snap)
	})
	return nil
}
