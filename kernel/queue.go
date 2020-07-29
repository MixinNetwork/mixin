package kernel

import (
	"github.com/MixinNetwork/mixin/common"
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
	chain := node.GetOrCreateChain(node.IdForNetwork)
	s := &common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: tx.PayloadHash(),
	}
	err = chain.AppendSelfEmpty(s)
	return tx.PayloadHash().String(), err
}

func (node *Node) LoadCacheToQueue() error {
	chain := node.GetOrCreateChain(node.IdForNetwork)
	return node.persistStore.CacheListTransactions(func(tx *common.VersionedTransaction) error {
		s := &common.Snapshot{
			Version:     common.SnapshotVersion,
			NodeId:      node.IdForNetwork,
			Transaction: tx.PayloadHash(),
		}
		return chain.AppendSelfEmpty(s)
	})
}

func (chain *Chain) ConsumeQueue() error {
	chain.QueuePollSnapshots(func(m *CosiAction) (bool, error) {
		if !chain.running {
			return false, nil
		}
		err := chain.cosiHandleAction(m)
		if err != nil {
			return false, err
		}
		if m.Action != CosiActionFinalization {
			return false, nil
		}
		if m.finalized || !m.WantTx || m.PeerId == chain.node.IdForNetwork {
			return m.finalized, nil
		}
		logger.Debugf("ConsumeQueue finalized snapshot without transaction %s %s %s\n", m.PeerId, m.SnapshotHash, m.Snapshot.Transaction)
		chain.node.Peer.SendTransactionRequestMessage(m.PeerId, m.Snapshot.Transaction)
		return m.finalized, nil
	})
	return nil
}
