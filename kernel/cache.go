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
	if node.transactionsFilter[hash] {
		return nil
	}
	node.transactionsFilter[hash] = true
	err := s.Transaction.Validate(node.store.SnapshotsGetUTXO, node.store.SnapshotsCheckGhost)
	if err != nil {
		return nil
	}

	if len(s.Signatures) == 0 {
		return node.validateBareSnapshot(s)
	}

	return node.consensusSnapshot(s)
}

func (node *Node) consensusSnapshot(s *common.Snapshot) error {
	refValid, err := common.VerifyReferences(s, nil)
	if err != nil || !refValid {
		return err
	}

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

	if validSigs > (len(node.ConsensusPeers)+1)/2*3 {
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

func (node *Node) validateBareSnapshot(s *common.Snapshot) error {
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
