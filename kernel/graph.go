package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) handleSnapshotInput(s *common.Snapshot) error {
	// if the transaction is a node accept, then create it with no references
	// and its node id should always be the new accepted node
	// ...
	// ...
	// check transaction in snapshot node graph again before final snapshot write
	node.clearConsensusSignatures(s)
	err := node.verifyTransactionInSnapshot(s)
	if err != nil {
		logger.Println("verifyTransactionInSnapshot ERROR", err)
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

	cache, final, err := node.verifySnapshot(s)
	if err != nil {
		return err
	}

	if !(s.RoundNumber == cache.Number || s.RoundNumber == final.Number && len(cache.Snapshots) == 0) {
		return nil
	}

	if node.verifyFinalization(s) {
		// so B created new cache round, and now A appended to an error round
		cache.Snapshots = append(cache.Snapshots, s)
		topo := &common.SnapshotWithTopologicalOrder{
			Snapshot:         *s,
			TopologicalOrder: node.TopoCounter.Next(),
		}
		err := node.store.WriteSnapshot(topo)
		if err != nil {
			return err
		}
		node.Graph.CacheRound[s.NodeId] = cache
		node.Graph.FinalRound[s.NodeId] = final
		return nil
	}

	err = s.LockInputs(node.store)
	if err != nil {
		logger.Println("LOCK INPUTS ERROR", err)
		return nil
	}
	node.signSnapshot(s)

	if node.IdForNetwork == s.NodeId {
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
	} else {
		// FIXME gossip peers are different from consensus nodes
		err := node.Peer.SendSnapshotMessage(s.NodeId, s)
		if err != nil {
			return err
		}
	}

	node.Graph.CacheRound[s.NodeId] = cache
	node.Graph.FinalRound[s.NodeId] = final
	return nil
}

func (node *Node) verifySnapshot(s *common.Snapshot) (*CacheRound, *FinalRound, error) {
	logger.Println("VERIFY SNAPSHOT", *s)
	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if osigs := node.SnapshotsPool[s.PayloadHash()]; len(osigs) > 0 || node.verifyFinalization(s) {
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
		return cache, final, nil
	}

	cacheStart, _ := cache.Gap()
	if s.Timestamp >= config.SnapshotRoundGap+cacheStart {
		final = cache.asFinal()
		cache = &CacheRound{
			NodeId: s.NodeId,
			Number: cache.Number + 1,
		}
	}

	return cache, final, nil
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
		references := [2]crypto.Hash{final.Hash, best.Hash}

		final = cache.asFinal()
		cache = &CacheRound{
			NodeId:     s.NodeId,
			Number:     final.Number + 1,
			References: references,
		}
		err := node.store.StartNewRound(s.NodeId, cache.Number, references, final.Start)
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

func (node *Node) verifyTransactionInSnapshot(s *common.Snapshot) error {
	txHash := s.Transaction.PayloadHash()
	in, err := node.store.CheckTransactionInNode(s.NodeId, txHash)
	if err != nil {
		return err
	} else if in {
		return fmt.Errorf("transaction %s already snapshot by node %s", txHash.String(), s.NodeId.String())
	}

	finalized, err := node.store.CheckTransactionFinalization(txHash)
	if err != nil {
		return err
	}
	if finalized && !node.verifyFinalization(s) {
		return fmt.Errorf("transaction %s already finalized, won't sign it any more", txHash.String())
	}
	if finalized {
		return nil
	}

	tx, err := node.store.ReadTransaction(txHash)
	if err != nil || tx != nil {
		return err
	}
	err = s.Transaction.Validate(node.store)
	if err != nil {
		return err
	}
	return node.store.WriteTransaction(&s.Transaction.Transaction)
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
