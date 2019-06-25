package kernel

import (
	"github.com/MixinNetwork/mixin/common"
)

func (node *Node) checkFinalSnapshotTransaction(s *common.Snapshot) (*common.VersionedTransaction, error) {
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return nil, err
	}

	tx, _, err := node.persistStore.ReadTransaction(s.Transaction)
	if err != nil || tx != nil {
		return tx, err
	}

	tx, err = node.persistStore.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return nil, err
	}

	err = tx.LockInputs(node.persistStore, true)
	if err != nil {
		return nil, err
	}

	if d := tx.DepositData(); d != nil {
		err = node.persistStore.WriteAsset(d.Asset())
		if err != nil {
			return nil, err
		}
	}
	return tx, node.persistStore.WriteTransaction(tx)
}

func (node *Node) tryToStartNewRound(s *common.Snapshot) error {
	if s.RoundNumber == 0 {
		return nil
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if s.RoundNumber != cache.Number+1 {
		return nil
	}

	if round, err := node.startNewRound(s, cache); err != nil {
		return err
	} else if round == nil {
		return nil
	} else {
		final = round
	}
	cache = &CacheRound{
		NodeId:     s.NodeId,
		Number:     s.RoundNumber,
		Timestamp:  s.Timestamp,
		References: s.References,
	}
	err := node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
	if err != nil {
		panic(err)
	}

	node.assignNewGraphRound(final, cache)
	return nil
}

func (node *Node) handleSyncFinalSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	if node.checkInitialAcceptSnapshot(s, tx) {
		err := node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
		return node.reloadConsensusNodesList(s, tx)
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if s.RoundNumber < cache.Number {
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return node.queueSnapshotOrPanic(s, true)
	}
	if s.RoundNumber == cache.Number && !s.References.Equal(cache.References) {
		if s.NodeId == node.IdForNetwork {
			return nil
		}
		if len(cache.Snapshots) != 0 {
			return nil
		}
		err := node.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, s.References)
		if err != nil {
			panic(err)
		}
		cache.References = s.References
		node.assignNewGraphRound(final, cache)
		return node.handleSnapshotInput(s)
	}
	if s.RoundNumber == cache.Number+1 {
		if round, err := node.startNewRound(s, cache); err != nil {
			return node.queueSnapshotOrPanic(s, true)
		} else if round == nil {
			return nil
		} else {
			final = round
		}
		cache = &CacheRound{
			NodeId:     s.NodeId,
			Number:     s.RoundNumber,
			Timestamp:  s.Timestamp,
			References: s.References,
		}
		err := node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
	}
	node.assignNewGraphRound(final, cache)

	if !cache.ValidateSnapshot(s, false) {
		return nil
	}
	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: node.TopoCounter.Next(),
	}
	err := node.persistStore.WriteSnapshot(topo)
	if err != nil {
		panic(err)
	}
	if !cache.ValidateSnapshot(s, true) {
		panic("should never be here")
	}
	node.assignNewGraphRound(final, cache)
	node.removeFromCache(s)
	return node.reloadConsensusNodesList(s, tx)
}
