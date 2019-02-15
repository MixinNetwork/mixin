package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) handleSnapshotInput(s *common.Snapshot) error {
	// if the transaction is a node accept, then create it with no references
	// and its node id should always be the new accepted node
	node.clearConsensusSignatures(s)
	err := node.verifyTransactionInSnapshot(s)
	if err != nil {
		return nil
	}

	defer node.Graph.UpdateFinalCache()

	err = node.tryToSignSnapshot(s)
	if err != nil {
		return err
	}
	if !node.verifySnapshotNodeSignature(s) {
		return nil
	}

	err = node.verifySnapshot(s)
	if err != nil {
		return err
	}

	if node.verifyFinalization(s) {
		return nil
	}
	if node.IdForNetwork != s.NodeId {
		// FIXME gossip peers are different from consensus nodes
		return node.Peer.SendSnapshotMessage(s.NodeId, s)
	}

	for _, cn := range node.ConsensusNodes {
		peerId := cn.Account.Hash().ForNetwork(node.networkId)
		cacheId := s.PayloadHash().ForNetwork(peerId)
		if time.Now().Before(node.ConsensusCache[cacheId].Add(time.Duration(config.SnapshotRoundGap))) {
			continue
		}
		err = node.Peer.SendSnapshotMessage(peerId, s)
		if err != nil {
			return err
		}
		node.ConsensusCache[cacheId] = time.Now()
	}
	return nil
}

func (node *Node) verifySnapshot(s *common.Snapshot) error {
	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if s.RoundNumber < cache.Number || s.RoundNumber > cache.Number+1 {
		return nil
	}
	if s.RoundNumber == cache.Number {
		if s.References[0] != cache.References[0] || s.References[1] != cache.References[1] {
			return nil
		}
	} else if s.RoundNumber == cache.Number+1 {
		round, err := node.verifyReferences(s, cache)
		if err != nil || round == nil {
			return err
		}
		final = round
		cache = &CacheRound{
			NodeId:     s.NodeId,
			Number:     s.RoundNumber,
			Timestamp:  s.Timestamp,
			References: s.References,
		}
		err = node.store.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			return err
		}
	}

	if osigs := node.SnapshotsPool[s.PayloadHash()]; len(osigs) > 0 {
		filter := make(map[crypto.Signature]bool)
		for _, sig := range s.Signatures {
			filter[sig] = true
		}
		for _, sig := range osigs {
			if filter[sig] {
				continue
			}
			s.Signatures = append(s.Signatures, sig)
			filter[sig] = true
		}
		node.SnapshotsPool[s.PayloadHash()] = append([]crypto.Signature{}, s.Signatures...)
	} else {
		node.signSnapshot(s)
	}
	if node.verifyFinalization(s) && cache.AddSnapshot(s) {
		topo := &common.SnapshotWithTopologicalOrder{
			Snapshot:         *s,
			TopologicalOrder: node.TopoCounter.Next(),
		}
		err := node.store.WriteSnapshot(topo)
		if err != nil {
			return err
		}
	}
	node.Graph.CacheRound[s.NodeId] = cache
	node.Graph.FinalRound[s.NodeId] = final
	return nil
}

func (node *Node) tryToSignSnapshot(s *common.Snapshot) error {
	if s.NodeId != node.IdForNetwork || len(s.Signatures) != 0 || s.Timestamp != 0 {
		return nil
	}
	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()
	for {
		s.Timestamp = uint64(time.Now().UnixNano())
		if s.Timestamp > cache.Timestamp {
			break
		}
		time.Sleep(1 * time.Millisecond)
	}

	cacheStart, _ := cache.Gap()
	if s.Timestamp >= config.SnapshotRoundGap+cacheStart {
		final = cache.asFinal()
		best := &FinalRound{NodeId: final.NodeId}
		for _, r := range node.Graph.FinalRound {
			if r.NodeId == s.NodeId || r.Start < best.Start {
				continue
			}
			if r.Start+config.SnapshotRoundGap < uint64(time.Now().UnixNano()) {
				best = r
			}
		}
		if best.NodeId == final.NodeId {
			panic(node.IdForNetwork)
		}

		cache = &CacheRound{
			NodeId:     s.NodeId,
			Number:     final.Number + 1,
			References: [2]crypto.Hash{final.Hash, best.Hash},
		}
		err := node.store.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
	}
	cache.Timestamp = s.Timestamp

	s.RoundNumber = cache.Number
	s.References = cache.References
	node.signSnapshot(s)
	node.Graph.CacheRound[s.NodeId] = cache
	node.Graph.FinalRound[s.NodeId] = final
	return nil
}

func (node *Node) verifyReferences(s *common.Snapshot, cache *CacheRound) (*FinalRound, error) {
	if s.RoundNumber != cache.Number+1 {
		return nil, nil
	}
	final := cache.asFinal()
	if final == nil {
		return nil, nil
	}
	if s.References[0] != final.Hash {
		err := cache.FilterByMask(node.store, s.References[0])
		if err != nil {
			return nil, err
		}
		final = cache.asFinal()
	}
	if s.References[0] != final.Hash {
		return nil, nil
	}

	external, err := node.store.ReadRound(s.References[1])
	if err != nil {
		return nil, err
	}
	if external == nil {
		return nil, nil
	}
	link, err := node.store.ReadLink(s.NodeId, external.NodeId)
	if external.Number >= link {
		return final, err
	}
	return nil, err
}

func (node *Node) verifyTransactionInSnapshot(s *common.Snapshot) error {
	in, err := node.store.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil {
		return err
	} else if in {
		return fmt.Errorf("transaction %s already snapshot by node %s", s.Transaction.String(), s.NodeId.String())
	}

	finalized, err := node.store.CheckTransactionFinalization(s.Transaction)
	if err != nil {
		return err
	}
	snapFinalized := node.verifyFinalization(s)
	if finalized && !snapFinalized {
		return fmt.Errorf("transaction %s already finalized, won't sign it any more", s.Transaction.String())
	}
	if finalized {
		return nil
	}

	tx, err := node.store.ReadTransaction(s.Transaction)
	if err != nil || tx != nil {
		return err
	}
	if !snapFinalized {
		err = s.SignedTransaction.Validate(node.store)
		if err != nil {
			return err
		}
	}
	err = s.LockInputs(node.store, snapFinalized)
	if err != nil {
		return err
	}
	return node.store.WriteTransaction(s.SignedTransaction)
}

func (node *Node) verifySnapshotNodeSignature(s *common.Snapshot) bool {
	msg := s.Payload()
	for _, cn := range node.ConsensusNodes {
		nodeId := cn.Account.Hash().ForNetwork(node.networkId)
		if nodeId != s.NodeId {
			continue
		}
		for _, sig := range s.Signatures {
			if cn.Account.PublicSpendKey.Verify(msg, sig) {
				return true
			}
		}
		break
	}
	return false
}

func (node *Node) clearConsensusSignatures(s *common.Snapshot) {
	msg := s.Payload()
	sigs := make([]crypto.Signature, 0)
	filter := make(map[crypto.Signature]bool)
	for _, sig := range s.Signatures {
		if filter[sig] {
			continue
		}
		for _, cn := range node.ConsensusNodes {
			if cn.Account.PublicSpendKey.Verify(msg, sig) {
				sigs = append(sigs, sig)
			}
		}
		filter[sig] = true
	}
	s.Signatures = sigs
}

func (node *Node) signSnapshot(s *common.Snapshot) {
	s.Sign(node.Account.PrivateSpendKey)
	node.clearConsensusSignatures(s)
	node.SnapshotsPool[s.PayloadHash()] = append([]crypto.Signature{}, s.Signatures...)
}

func (node *Node) verifyFinalization(s *common.Snapshot) bool {
	consensusThreshold := len(node.ConsensusNodes) * 2 / 3
	return len(s.Signatures) > consensusThreshold
}
