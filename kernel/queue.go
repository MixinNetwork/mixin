package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/util"
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
	chain := node.GetOrCreateChain(node.IdForNetwork)
	err = chain.QueueAppendSnapshot(node.IdForNetwork, &common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: tx.PayloadHash(),
	}, false)
	return tx.PayloadHash().String(), err
}

func (node *Node) LoadCacheToQueue() error {
	chain := node.GetOrCreateChain(node.IdForNetwork)
	return node.persistStore.CacheListTransactions(func(tx *common.VersionedTransaction) error {
		return chain.QueueAppendSnapshot(node.IdForNetwork, &common.Snapshot{
			Version:     common.SnapshotVersion,
			NodeId:      node.IdForNetwork,
			Transaction: tx.PayloadHash(),
		}, false)
	})
}

func (chain *Chain) ConsumeQueue() error {
	period := time.Second
	timer := util.NewTimer(period)
	defer timer.Stop()

	chain.QueuePollSnapshots(func(peerId crypto.Hash, snap *common.Snapshot) error {
		m := &CosiAction{PeerId: peerId, Snapshot: snap}
		if snap.Version == 0 {
			m.Action = CosiActionFinalization
			m.Snapshot.Hash = snap.PayloadHash()
		} else if snap.Signature != nil {
			m.Action = CosiActionFinalization
			m.Snapshot.Hash = snap.PayloadHash()
		} else if snap.NodeId != chain.node.IdForNetwork {
			m.Action = CosiActionExternalAnnouncement
			m.Snapshot.Hash = snap.PayloadHash()
		} else {
			m.Action = CosiActionSelfEmpty
		}

		if m.Action != CosiActionFinalization {
			chain.cosiActionsChan <- m
			return nil
		}

		tx, err := chain.persistStore.CacheGetTransaction(snap.Transaction)
		if err != nil {
			return err
		}
		if tx != nil {
			chain.cosiActionsChan <- m
			return nil
		}

		tx, _, err = chain.persistStore.ReadTransaction(snap.Transaction)
		if err != nil {
			return err
		}
		if tx != nil {
			chain.cosiActionsChan <- m
			return nil
		}

		if peerId == chain.node.IdForNetwork {
			return nil
		}
		logger.Debugf("ConsumeQueue finalized snapshot without transaction %s %s %s\n", peerId, snap.Hash, snap.Transaction)
		chain.node.Peer.SendTransactionRequestMessage(peerId, snap.Transaction, timer)
		return chain.QueueAppendSnapshot(peerId, snap, true)
	})
	return nil
}
