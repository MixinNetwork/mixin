package kernel

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
)

func (node *Node) checkFinalSnapshotTransaction(s *common.Snapshot) (bool, error) {
	inNode, err := node.store.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return false, err
	}

	tx, err := node.store.ReadTransaction(s.Transaction)
	if err != nil || tx != nil {
		return true, err
	}

	tx, err = node.store.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return false, err
	}

	err = tx.LockInputs(node.store, true)
	if err != nil {
		return false, err
	}

	return true, node.store.WriteTransaction(tx)
}

func (node *Node) handleSyncFinalSnapshot(s *common.Snapshot) error {
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
		node.Graph.CacheRound[s.NodeId] = cache
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

	node.Graph.CacheRound[s.NodeId] = cache
	node.Graph.FinalRound[s.NodeId] = final
	return nil
}

func (node *Node) startNewRound(s *common.Snapshot, cache *CacheRound) (*FinalRound, error) {
	if s.RoundNumber != cache.Number+1 {
		panic("should never be here")
	}
	final := cache.asFinal()
	if final == nil {
		return nil, fmt.Errorf("self cache snapshots not collected yet")
	}
	if s.References.Self != final.Hash {
		return nil, fmt.Errorf("self cache snapshots not match yet")
	}

	external, err := node.store.ReadRound(s.References.External)
	if err != nil {
		return nil, err
	}
	if external == nil {
		return nil, fmt.Errorf("external round not collected yet")
	}
	if final.NodeId == external.NodeId {
		return nil, nil
	}

	link, err := node.store.ReadLink(s.NodeId, external.NodeId)
	if external.Number >= link {
		return final, err
	}
	return nil, err
}
