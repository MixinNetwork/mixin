package kernel

import (
	"github.com/MixinNetwork/mixin/common"
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

func (chain *Chain) clearAndQueueSnapshotOrPanic(s *common.Snapshot) error {
	if chain.ChainId != s.NodeId {
		panic("should never be here")
	}
	return chain.AppendSelfEmpty(&common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      s.NodeId,
		Transaction: s.Transaction,
	})
}
