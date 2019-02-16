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
	snap := &common.Snapshot{
		Transaction: tx.PayloadHash(),
	}
	return tx.PayloadHash().String(), store.QueueAppendSnapshot(crypto.Hash{}, snap)
}

func (node *Node) ConsumeQueue() error {
	var offset = uint64(0)
	filter := make(map[crypto.Hash]time.Time)
	for {
		err := node.store.QueuePollSnapshots(offset, func(off uint64, peerId crypto.Hash, snap *common.Snapshot) error {
			tx, err := node.store.CacheGetTransaction(snap.Transaction)
			if err != nil {
				return err
			}
			if tx != nil {
				node.mempoolChan <- snap
				offset = off
				return nil
			}

			offset = off
			if peerId == node.IdForNetwork || !peerId.HasValue() {
				return nil
			}
			hash := snap.Transaction.ForNetwork(peerId)
			if filter[hash].Add(time.Duration(config.SnapshotRoundGap)).After(time.Now()) {
				node.Peer.SendTransactionRequestMessage(peerId, snap.Transaction)
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}
