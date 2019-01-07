package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) feedMempool(s *common.Snapshot) error {
	node.mempoolChan <- s
	return nil
}

func (node *Node) ConsumeMempool() error {
	for {
		select {
		case s := <-node.mempoolChan:
			err := node.handleSnapshotInput(s)
			if err != nil {
				return err
			}
		}
	}
}

func (node *Node) clearConsensusSignatures(s *common.Snapshot) {
	msg := s.Payload()
	sigs := make([]crypto.Signature, 0)
	filter := make(map[crypto.Signature]bool)
	for _, sig := range s.Signatures {
		if filter[sig] {
			continue
		}
		for _, n := range node.ConsensusNodes {
			if n.PublicSpendKey.Verify(msg, sig) {
				sigs = append(sigs, sig)
			}
		}
		filter[sig] = true
	}
	s.Signatures = sigs
}

func (node *Node) handleSnapshotInput(s *common.Snapshot) error {
	err := s.Transaction.Validate(node.store.SnapshotsReadUTXO, node.store.SnapshotsCheckGhost)
	if err != nil {
		logger.Println("VALIDATE TRANSACTION", err)
		return nil
	}

	defer node.Graph.UpdateFinalCache()
	node.clearConsensusSignatures(s)

	if len(s.Signatures) == 0 {
		err = node.signSnapshot(s)
	} else {
		err = node.verifySnapshot(s)
	}

	if err != nil {
		return err
	}
	return s.LockInputs(node.store.SnapshotsLockUTXO)
}

func (node *Node) verifyReferences(self FinalRound, s *common.Snapshot) (map[crypto.Hash]uint64, bool, error) {
	links := make(map[crypto.Hash]uint64)
	if len(s.References) != 2 {
		return links, true, fmt.Errorf("invalid reference count %d", len(s.References))
	}
	ref0, ref1 := s.References[0], s.References[1]
	if ref0 == ref1 {
		return links, true, fmt.Errorf("same references %s", s.Transaction.PayloadHash().String())
	}

	if ref0 != self.Hash {
		return links, true, fmt.Errorf("invalid self reference %s", s.Transaction.PayloadHash().String())
	}
	if s.NodeId != self.NodeId {
		panic(*s)
	}

	for _, final := range node.Graph.FinalRound {
		if final.NodeId == s.NodeId || final.Hash != ref1 {
			continue
		}
		links[self.NodeId] = self.Number
		links[final.NodeId] = final.Number
		selfLink, err := node.store.SnapshotsReadRoundLink(s.NodeId, self.NodeId)
		if err != nil {
			return links, false, err
		}
		if links[self.NodeId] < selfLink {
			return links, true, fmt.Errorf("invalid self reference %d=>%d", selfLink, links[self.NodeId])
		}
		finalLink, err := node.store.SnapshotsReadRoundLink(s.NodeId, final.NodeId)
		if err != nil {
			return links, false, err
		}
		if links[final.NodeId] < finalLink {
			return links, true, fmt.Errorf("invalid final reference %d=>%d", finalLink, links[final.NodeId])
		}
		return links, true, nil
	}
	return links, true, fmt.Errorf("invalid references %s", s.Transaction.PayloadHash().String())
}

func (node *Node) verifyFinalization(s *common.Snapshot) bool {
	if !s.CheckSignature(node.Account.PublicSpendKey) {
		s.Sign(node.Account.PrivateSpendKey)
	}

	consensusThreshold := len(node.ConsensusNodes) * 2 / 3
	return len(s.Signatures) > consensusThreshold
}

func (node *Node) verifySnapshot(s *common.Snapshot) error {
	logger.Println("VERIFY SNAPSHOT", *s)
	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()
	if s.RoundNumber < cache.Number {
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return nil
	}
	if s.Timestamp < cache.End {
		return nil
	}
	if s.Timestamp-cache.Start >= config.SnapshotRoundGap {
		if len(cache.Snapshots) == 0 {
			cache.Start = s.Timestamp
		} else {
			for _, ps := range cache.Snapshots {
				if !node.verifyFinalization(ps) {
					return nil
				}
			}

			final = cache.asFinal()
			cache = &CacheRound{
				NodeId: s.NodeId,
				Number: cache.Number + 1,
				Start:  s.Timestamp,
				End:    s.Timestamp,
			}
		}
	}

	links, handled, err := node.verifyReferences(*final, s)
	if err != nil {
		logger.Println(err)
		if !handled {
			return err
		}
		return nil
	}

	if o := node.SnapshotsPool[s.Transaction.PayloadHash()]; o != nil {
		filter := make(map[crypto.Signature]bool)
		for _, sig := range s.Signatures {
			filter[sig] = true
		}
		for _, sig := range o.Signatures {
			if filter[sig] {
				continue
			}
			s.Signatures = append(s.Signatures, sig)
			filter[sig] = true
		}
	}
	node.SnapshotsPool[s.Transaction.PayloadHash()] = s

	if node.verifyFinalization(s) {
		if s.RoundNumber == cache.Number+1 {
			final = cache.asFinal()
			cache = snapshotAsCacheRound(s)
		} else {
			cache.Snapshots = append(cache.Snapshots, s)
			cache.End = s.Timestamp
		}
		topo := &common.SnapshotWithTopologicalOrder{
			Snapshot:         *s,
			TopologicalOrder: node.TopoCounter.Next(),
			RoundLinks:       links,
		}
		err := node.store.SnapshotsWriteSnapshot(topo)
		if err != nil {
			return err
		}
	} else if node.IdForNetwork != s.NodeId {
		// FIXME gossip peers are different from consensus nodes
		err := node.Peer.Neighbors[s.NodeId].SendSnapshotMessage(s)
		if err != nil {
			return err
		}
	}

	node.Graph.CacheRound[s.NodeId] = cache
	node.Graph.FinalRound[s.NodeId] = final
	return nil
}

func (node *Node) signSnapshot(s *common.Snapshot) error {
	if s.NodeId != node.IdForNetwork {
		return nil
	}
	logger.Println("SIGN SNAPSHOT", *s)

	round := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	for {
		s.Timestamp = uint64(time.Now().UnixNano())
		if s.Timestamp > round.End {
			break
		}
		time.Sleep(1 * time.Millisecond)
	}
	if s.Timestamp-round.Start >= config.SnapshotRoundGap {
		if len(round.Snapshots) == 0 {
			round.Start = s.Timestamp
		} else {
			for _, ps := range round.Snapshots {
				if !node.verifyFinalization(ps) {
					panic("should queue if pending round full")
				}
			}

			final = round.asFinal()
			round = &CacheRound{
				NodeId: s.NodeId,
				Number: round.Number + 1,
				Start:  s.Timestamp,
			}
		}
	}
	round.End = s.Timestamp

	best := &FinalRound{}
	for _, r := range node.Graph.FinalRound {
		if r.NodeId != s.NodeId && r.Start >= best.Start && r.End < uint64(time.Now().UnixNano()) {
			best = r
		}
	}
	if best.NodeId == final.NodeId {
		panic(node.IdForNetwork)
	}

	s.RoundNumber = round.Number
	s.References = [2]crypto.Hash{final.Hash, best.Hash}
	s.Sign(node.Account.PrivateSpendKey)

	for _, p := range node.Peer.Neighbors {
		err := p.SendSnapshotMessage(s)
		if err != nil {
			return err
		}
	}
	node.SnapshotsPool[s.Transaction.PayloadHash()] = s
	node.Graph.CacheRound[s.NodeId] = round
	node.Graph.FinalRound[s.NodeId] = final
	logger.Println(node.Graph.Print())
	return nil
}
