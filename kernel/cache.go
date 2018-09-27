package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) feedMempool(s *common.Snapshot) error {
	node.mempoolChan <- s
	return nil
}

func (node *Node) ConsumeMempool() error {
	for {
		if !node.syncrhoinized {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		select {
		case s := <-node.mempoolChan:
			err := node.handleSnapshotInput(s)
			if err != nil {
				return err
			}
		}
	}
}

func (node *Node) handleSnapshotInput(s *common.Snapshot) error {
	err := s.Transaction.Validate(node.store.SnapshotsGetUTXO, node.store.SnapshotsCheckGhost)
	if err != nil {
		logger.Println("VALIDATE TRANSACTION", err)
		return nil
	}

	if len(s.Signatures) == 0 {
		return node.signSnapshot(s)
	}

	return node.verifySnapshot(s)
}

// TODO node a must not reference round n-1 from node b if a has referenced n
func (node *Node) verifyReferences(s *common.Snapshot) bool {
	if len(s.References) != 2 {
		return false
	}
	ref0, ref1 := s.References[0], s.References[1]
	if ref0 == ref1 {
		return false
	}

	if ref0 != node.Graph.FinalRound[s.NodeId].Hash {
		return false
	}

	for _, final := range node.Graph.FinalRound {
		if final.Hash == ref1 {
			return true
		}
	}
	return false
}

func (node *Node) verifyFinalization(s *common.Snapshot) bool {
	var validSigs int
	for _, p := range node.ConsensusPeers {
		if common.CheckSignature(s, p.Account.PublicSpendKey) {
			validSigs = validSigs + 1
		}
	}

	if !common.CheckSignature(s, node.Account.PublicSpendKey) {
		common.SignSnapshot(s, node.Account.PrivateSpendKey)
	}
	validSigs = validSigs + 1

	consensusThreshold := (len(node.ConsensusPeers)+1)*2/3 + 1
	return validSigs >= consensusThreshold
}

func (node *Node) verifySnapshot(s *common.Snapshot) error {
	logger.Println("VERIFY SNAPSHOT", *s)
	cache := node.Graph.CacheRound[s.NodeId]
	if s.RoundNumber < cache.Number {
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return nil
	}
	if s.Timestamp < cache.End {
		return nil
	}
	if s.Timestamp-cache.Start >= common.SnapshotRoundGap {
		if len(cache.Snapshots) == 0 {
			cache.Start = s.Timestamp
		} else {
			for _, ps := range cache.Snapshots {
				if !node.verifyFinalization(ps) {
					return nil
				}
			}

			node.Graph.FinalRound[s.NodeId] = cache.asFinal()
			cache = &CacheRound{
				NodeId: s.NodeId,
				Number: cache.Number + 1,
				Start:  s.Timestamp,
				End:    s.Timestamp,
			}
			node.Graph.CacheRound[s.NodeId] = cache
		}
	}

	if !node.verifyReferences(s) {
		return nil
	}

	if o := node.SnapshotsPool[s.Transaction.Hash()]; o != nil {
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
	node.SnapshotsPool[s.Transaction.Hash()] = s

	if node.verifyFinalization(s) {
		if s.RoundNumber == cache.Number+1 {
			node.Graph.FinalRound[s.NodeId] = cache.asFinal()
			node.Graph.CacheRound[s.NodeId] = snapshotAsCacheRound(s)
		} else {
			cache.Snapshots = append(cache.Snapshots, s)
			cache.End = s.Timestamp
		}
		topo := &common.SnapshotWithTopologicalOrder{
			Snapshot:         *s,
			TopologicalOrder: node.TopoCounter.Next(),
		}
		return node.store.SnapshotsWrite(topo)
	}

	if node.IdForNetwork != s.NodeId {
		msg := buildSnapshotMessage(s)
		return node.ConsensusPeers[s.NodeId].Send(msg)
	}
	return nil
}

func (node *Node) signSnapshot(s *common.Snapshot) error {
	if s.NodeId != node.IdForNetwork {
		return nil
	}

	round := node.Graph.CacheRound[s.NodeId]
	final := node.Graph.FinalRound[s.NodeId]

	for {
		s.Timestamp = uint64(time.Now().UnixNano())
		if s.Timestamp > round.End {
			break
		}
		time.Sleep(1 * time.Millisecond)
	}
	if s.Timestamp-round.Start >= common.SnapshotRoundGap {
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
	s.References = []crypto.Hash{final.Hash, best.Hash}
	common.SignSnapshot(s, node.Account.PrivateSpendKey)

	node.Graph.CacheRound[s.NodeId] = round
	node.Graph.FinalRound[s.NodeId] = final
	logger.Println(node.Graph.Print())

	for _, p := range node.ConsensusPeers {
		err := p.Send(buildSnapshotMessage(s))
		if err != nil {
			return err
		}
	}
	node.SnapshotsPool[s.Transaction.Hash()] = s
	return nil
}
