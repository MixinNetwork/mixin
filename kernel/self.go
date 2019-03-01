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

	if tx.CheckMint() {
		err = node.validateMintTransaction(tx)
		if err != nil {
			return nil, nil
		}
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
		return nil
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

func (node *Node) determinBestRound() *FinalRound {
	var best *FinalRound
	var start, height uint64
	for id, rounds := range node.Graph.RoundHistory {
		if rc := len(rounds) - config.SnapshotReferenceThreshold; rc > 0 {
			rounds = append([]*FinalRound{}, rounds[rc:]...)
		}
		node.Graph.RoundHistory[id] = rounds
		rts, rh := rounds[0].Start, uint64(len(rounds))
		if rounds[0].NodeId == node.IdForNetwork || rh < height {
			continue
		}
		if rts+config.SnapshotRoundGap*rh > uint64(time.Now().UnixNano()) {
			continue
		}
		if rh > height || rts > start {
			best = rounds[0]
			start, height = rts, rh
		}
	}
	if best != nil {
		return best
	}
	for _, r := range node.Graph.FinalRound {
		if r.NodeId != node.IdForNetwork {
			return r
		}
	}
	return nil
}

func (node *Node) signSelfSnapshot(s *common.Snapshot, tx *common.SignedTransaction) error {
	if s.NodeId != node.IdForNetwork || len(s.Signatures) != 0 || s.Timestamp != 0 {
		panic("should never be here")
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	for {
		s.Timestamp = uint64(time.Now().UnixNano())
		if s.Timestamp > cache.Timestamp {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	if start, _ := cache.Gap(); s.Timestamp >= start+config.SnapshotRoundGap {
		best := node.determinBestRound()
		if best.NodeId == final.NodeId {
			panic("should never be here")
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
	node.Graph.RoundHistory[s.NodeId] = append(node.Graph.RoundHistory[s.NodeId], final.Copy())
	node.signSnapshot(s)
	s.Signatures = []*crypto.Signature{node.SignaturesPool[s.Hash]}
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
