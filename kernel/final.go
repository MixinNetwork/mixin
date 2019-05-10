package kernel

import (
	"github.com/MixinNetwork/mixin/common"
)

func (node *Node) checkFinalSnapshotTransaction(s *common.Snapshot) (*common.VersionedTransaction, error) {
	inNode, err := node.store.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return nil, err
	}

	tx, err := node.store.ReadTransaction(s.Transaction)
	if err != nil || tx != nil {
		return tx, err
	}

	tx, err = node.store.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return nil, err
	}

	err = tx.LockInputs(node.store, true)
	if err != nil {
		return nil, err
	}

	if d := tx.DepositData(); d != nil {
		err = node.store.WriteAsset(d.Asset())
		if err != nil {
			return nil, err
		}
	}
	return tx, node.store.WriteTransaction(tx)
}

func (node *Node) tryToStartNewRound(s *common.Snapshot) error {
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
	err := node.store.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
	if err != nil {
		panic(err)
	}

	node.assignNewGraphRound(final, cache)
	return nil
}

func (node *Node) handleSyncFinalSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
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
		err := node.store.UpdateEmptyHeadRound(cache.NodeId, cache.Number, s.References)
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
		err := node.store.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
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
	err := node.store.WriteSnapshot(topo)
	if err != nil {
		panic(err)
	}
	if !cache.ValidateSnapshot(s, true) {
		panic("should never be here")
	}
	node.assignNewGraphRound(final, cache)
	node.removeFromCache(s)
	return node.manageConsensusNodesList(tx)
}
