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
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: tx.PayloadHash(),
	}, false)
	return tx.PayloadHash().String(), err
}

func (node *Node) LoadCacheToQueue() error {
	return node.persistStore.CacheListTransactions(func(tx *common.VersionedTransaction) error {
		return node.QueueAppendSnapshot(node.IdForNetwork, &common.Snapshot{
			Version:     common.SnapshotVersion,
			NodeId:      node.IdForNetwork,
			Transaction: tx.PayloadHash(),
		}, false)
	})
}

func (node *Node) ConsumeQueue() error {
	node.persistStore.QueuePollSnapshots(func(peerId crypto.Hash, snap *common.Snapshot) error {
		m := &CosiAction{PeerId: peerId, Snapshot: snap}
		if snap.Version == 0 {
			m.Action = CosiActionFinalization
			m.Snapshot.Hash = snap.PayloadHash()
		} else if snap.Signature != nil {
			m.Action = CosiActionFinalization
			m.Snapshot.Hash = snap.PayloadHash()
		} else if snap.NodeId != node.IdForNetwork {
			m.Action = CosiActionExternalAnnouncement
			m.Snapshot.Hash = snap.PayloadHash()
		} else {
			m.Action = CosiActionSelfEmpty
		}

		if m.Action != CosiActionFinalization {
			node.cosiActionsChan <- m
			return nil
		}

		tx, err := node.persistStore.CacheGetTransaction(snap.Transaction)
		if err != nil {
			return err
		}
		if tx != nil {
			node.cosiActionsChan <- m
			return nil
		}

		tx, _, err = node.persistStore.ReadTransaction(snap.Transaction)
		if err != nil {
			return err
		}
		if tx != nil {
			node.cosiActionsChan <- m
			return nil
		}

		if peerId == node.IdForNetwork {
			return nil
		}
		node.Peer.SendTransactionRequestMessage(peerId, snap.Transaction)
		return node.QueueAppendSnapshot(peerId, snap, true)
	})
	return nil
}
