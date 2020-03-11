package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
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

func (node *Node) DumpAndClearCache(dump int64) error {
	node.persistStore.DumpAndClearCache(func(peerId crypto.Hash, snap *common.Snapshot) error {
		if dump > 0 {
			action := "CosiActionUNKNOWN"
			if snap.Version == 0 {
				panic("should never be here")
			} else if snap.Signature != nil {
				action = "CosiActionFinalization"
			} else if snap.NodeId != node.IdForNetwork {
				action = "CosiActionExternalAnnouncement"
			} else {
				action = "CosiActionSelfEmpty"
			}
			logger.Printf("DUMP %s %s\n", peerId, action)
		}
		dump--
		return nil
	})
	return nil
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

		if m.Action == CosiActionExternalAnnouncement {
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
		finalized := m.Action == CosiActionFinalization
		node.Peer.SendTransactionRequestMessage(peerId, snap.Transaction)
		return node.QueueAppendSnapshot(peerId, snap, finalized)
	})
	return nil
}
