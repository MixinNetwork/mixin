package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
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
	hash := s.Transaction.Hash()
	if !node.transactionsFilter[hash] {
		err := s.Transaction.Validate(node.store.SnapshotsGetUTXO, node.store.SnapshotsCheckGhost)
		if err != nil {
			return nil
		}
	}
	node.transactionsFilter[hash] = true

	if len(s.Signatures) == 0 {
		return node.signSnapshot(s)
	}

	return node.verifySnapshot(s)
}

func (node *Node) verifyReferences(s *common.Snapshot) bool {
	if len(s.References) != 2 {
		return false
	}
	if s.References[0].String() == s.References[1].String() {
		return false
	}

	filter := make(map[crypto.Hash]bool)
	for _, final := range node.Graph.FinalRound {
		filter[final.Hash] = true
	}
	return filter[s.References[0]] && filter[s.References[1]]
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

	consensusThreshold := (len(node.ConsensusPeers)+1)/2*3 + 1
	return validSigs >= consensusThreshold
}

func (node *Node) verifySnapshot(s *common.Snapshot) error {
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
		return nil
	}

	if !node.verifyReferences(s) {
		return nil
	}

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

	for _, p := range node.ConsensusPeers {
		err := p.Send(buildSnapshotMessage(s))
		if err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) signSnapshot(s *common.Snapshot) error {
	if s.NodeId.String() != node.IdForNetwork.String() {
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
			final = round.asFinal()
			round = &CacheRound{
				NodeId: s.NodeId,
				Number: round.Number + 1,
				Start:  s.Timestamp,
			}
		}
	}
	round.End = s.Timestamp

	best := final
	for _, r := range node.Graph.FinalRound {
		if r.Start >= best.Start && r.NodeId.String() != s.NodeId.String() {
			best = r
		}
	}
	if best.NodeId.String() == node.IdForNetwork.String() {
		panic(node.IdForNetwork.String())
	}

	s.RoundNumber = round.Number
	s.References = []crypto.Hash{final.Hash, best.Hash}
	common.SignSnapshot(s, node.Account.PrivateSpendKey)

	node.Graph.CacheRound[s.NodeId] = round
	node.Graph.FinalRound[s.NodeId] = final

	for _, p := range node.ConsensusPeers {
		err := p.Send(buildSnapshotMessage(s))
		if err != nil {
			return err
		}
	}
	return nil
}
