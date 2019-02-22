package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) checkCacheSnapshotTransaction(s *common.Snapshot) (*common.SignedTransaction, error) {
	inNode, err := node.store.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return nil, err
	}

	finalized, err := node.store.CheckTransactionFinalization(s.Transaction)
	if err != nil || finalized {
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

	err = tx.Validate(node.store)
	if err != nil {
		return nil, nil
	}

	err = tx.LockInputs(node.store, false)
	if err != nil {
		return nil, err
	}

	return tx, node.store.WriteTransaction(tx)
}

func (node *Node) collectSelfSignatures(s *common.Snapshot) error {
	if s.NodeId != node.IdForNetwork || len(s.Signatures) != 1 {
		panic("should never be here")
	}
	if len(node.SnapshotsPool[s.Hash]) == 0 || node.SignaturesPool[s.Hash] == nil {
		panic("should never be here")
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	if s.RoundNumber < cache.Number {
		return node.clearAndQueueSnapshotOrPanic(s)
	}
	if !cache.ValidateSnapshot(s, false) {
		return node.clearAndQueueSnapshotOrPanic(s)
	}

	filter := make(map[string]bool)
	osigs := node.SnapshotsPool[s.Hash]
	for _, sig := range osigs {
		filter[sig.String()] = true
	}
	for _, sig := range s.Signatures {
		if filter[sig.String()] {
			continue
		}
		osigs = append(osigs, sig)
		filter[sig.String()] = true
	}
	node.SnapshotsPool[s.Hash] = append([]*crypto.Signature{}, osigs...)

	if !node.verifyFinalization(osigs) {
		return nil
	}

	s.Signatures = append([]*crypto.Signature{}, osigs...)
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

	for peerId, _ := range node.ConsensusNodes {
		err := node.Peer.SendSnapshotMessage(peerId, s, 1)
		if err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) handleSelfFreshSnapshot(s *common.Snapshot, tx *common.SignedTransaction) error {
	err := node.signSelfSnapshot(s)
	if err != nil || len(s.Signatures) == 0 {
		return err
	}

	for peerId, _ := range node.ConsensusNodes {
		err := node.Peer.SendTransactionMessage(peerId, tx)
		if err != nil {
			return err
		}
		err = node.Peer.SendSnapshotMessage(peerId, s, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) signSelfSnapshot(s *common.Snapshot) error {
	if s.NodeId != node.IdForNetwork || len(s.Signatures) != 0 || s.Timestamp != 0 {
		panic("should never be here")
	}
	if len(node.SnapshotsPool[s.Hash]) > 0 || node.SignaturesPool[s.Hash] != nil {
		panic("should never be here")
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	for {
		s.Timestamp = uint64(time.Now().UnixNano())
		if s.Timestamp > cache.Timestamp {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if start, _ := cache.Gap(); s.Timestamp >= start+config.SnapshotRoundGap {
		best := &FinalRound{}
		for _, r := range node.Graph.FinalRound {
			if r.NodeId == s.NodeId || r.Start < best.Start {
				continue
			}
			if r.Start+config.SnapshotRoundGap < uint64(time.Now().UnixNano()) {
				best = r
			}
		}
		if !best.NodeId.HasValue() || best.NodeId == final.NodeId {
			panic("FIXME it is possible that no best available")
		}

		final = cache.asFinal()
		cache = &CacheRound{
			NodeId: s.NodeId,
			Number: final.Number + 1,
			References: &common.RoundLink{
				Self:     final.Hash,
				External: best.Hash,
			},
		}
		err := node.store.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
	}
	cache.Timestamp = s.Timestamp

	s.RoundNumber = cache.Number
	s.References = cache.References
	node.Graph.CacheRound[s.NodeId] = cache
	node.Graph.FinalRound[s.NodeId] = final
	node.signSnapshot(s)
	return nil
}
