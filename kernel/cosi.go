package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

const (
	CosiActionSelfEmpty = iota
	CosiActionSelfCommitment
	CosiActionSelfResponse
	CosiActionExternalAnnouncement
	CosiActionExternalChallenge
	CosiActionFinalization
)

type CosiAction struct {
	Action       int
	PeerId       crypto.Hash
	SnapshotHash crypto.Hash
	Snapshot     *common.Snapshot
	Commitment   *crypto.Key
	Signature    *crypto.CosiSignature
	Response     *[32]byte
	Transaction  *common.VersionedTransaction
	WantTx       bool
}

func (node *Node) CosiLoop() error {
	for {
		select {
		case m := <-node.cosiActionsChan:
			err := node.cosiHandleAction(m)
			if err != nil {
				return err
			}
		}
	}
}

func (node *Node) cosiHandleAction(m *CosiAction) error {
	switch m.Action {
	case CosiActionSelfEmpty:
		return node.sendCosiAnnouncement(m)
	case CosiActionSelfCommitment:
		return node.cosiHandleCommitment(m)
	case CosiActionSelfResponse:
		return node.cosiHandleResponse(m)
	case CosiActionExternalAnnouncement:
		return node.cosiHandleAnnouncement(m)
	case CosiActionExternalChallenge:
		return node.cosiHandleChallenge(m)
	case CosiActionFinalization:
		return node.handleFinalization(m)
	}
	return nil
}

func (node *Node) sendCosiAnnouncement(m *CosiAction) error {
	for peerId, _ := range node.ConsensusNodes {
		err := node.Peer.SendSnapshotAnnouncementMessage(peerId, m.Snapshot)
		if err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) cosiHandleAnnouncement(m *CosiAction) error {
	s := m.Snapshot
	validate(s)
	node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.PayloadHash(), R, wantTx)
	return nil
}

func (node *Node) cosiHandleChallenge(m *CosiAction) error {
	return nil
}

func (node *Node) cosiHandleResponse(m *CosiAction) error {
	return nil
}

func (node *Node) handleFinalization(m *CosiAction) error {
	return nil
}

func (node *Node) CosiQueueExternalAnnouncement(peerId crypto.Hash, s *common.Snapshot) error {
	if s.Version != common.SnapshotVersion {
		return nil
	}
	if s.NodeId == node.IdForNetwork || s.NodeId != peerId || s.Signature == nil {
		return nil
	}
	cn := node.ConsensusNodes[s.NodeId]
	if cn == nil {
		return nil
	}
	if !node.CacheVerify(s.PayloadHash(), s.Signature.Signature, cn.Signer.PublicSpendKey) {
		return nil
	}
	return node.QueueAppendSnapshot(peerId, s, false)
}

func (node *Node) CosiAggregateSelfCommitments(peerId crypto.Hash, snap crypto.Hash, commitment *crypto.Key, wantTx bool) error {
	m := &CosiAction{
		Action:       CosiActionSelfCommitment,
		SnapshotHash: snap,
		Commitment:   commitment,
		WantTx:       wantTx,
	}
	node.cosiActionsChan <- m
	return nil
}

func (node *Node) cosiHandleCommitment(m *CosiAction) error {
	pool := node.SnapshotsPool[node.IdForNetwork]
	ann := pool[m.SnapshotHash]
	if ann == nil {
		return nil
	}
	cn := node.ConsensusNodes[m.PeerId]
	if cn == nil {
		return nil
	}
	for i, n := range node.SortedConsensusNodes {
		if n.IdForNetwork == m.PeerId {
			ann.Masks = append(ann.Masks, i)
			ann.Commitments = append(ann.Commitments, m.Commitment)
			ann.WantTxs[m.PeerId] = m.WantTx
			break
		}
	}
	if len(ann.Commitments) < node.ConsensusBase(ann.Timestamp) {
		return nil
	}
	cosi, err := crypto.CosiAggregateCommitment(ann.Commitments, ann.Masks)
	if err != nil {
		return err
	}
	for id, _ := range node.ConsensusNodes {
		if ann.WantTxs[id] {
			node.Peer.SendTransactionChallengeMessage(id, snap, cosi, tx)
		} else {
			node.Peer.SendTransactionChallengeMessage(id, snap, cosi, nil)
		}
	}
}

func (node *Node) CosiQueueExternalChallenge(peerId crypto.Hash, snap crypto.Hash, cosi *crypto.CosiSignature, ver *common.VersionedTransaction) error {
	m := &CosiAction{
		Action:       CosiActionExternalChallenge,
		SnapshotHash: snap,
		Signature:    cosi,
		Transaction:  ver,
	}
	node.cosiActionsChan <- m
	return nil
}

func (node *Node) CosiAggregateSelfResponses(peerId crypto.Hash, snap crypto.Hash, response *[32]byte) error {
	m := &CosiAction{
		Action:       CosiActionSelfResponse,
		SnapshotHash: snap,
		Response:     response,
	}
	node.cosiActionsChan <- m
	return nil
}

func (node *Node) VerifyAndQueueAppendSnapshotFinalization(peerId crypto.Hash, s *common.Snapshot) error {
	s.Hash = s.PayloadHash()
	if !node.verifyFinalization(s.Timestamp, s.Signatures) {
		return nil
	}
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil {
		return err
	}
	if inNode {
		node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash)
		return node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash)
	}

	sigs := make([]*crypto.Signature, 0)
	signaturesFilter := make(map[string]bool)
	signersMap := make(map[crypto.Hash]bool)
	for i, sig := range s.Signatures {
		s.Signatures[i] = nil
		if signaturesFilter[sig.String()] {
			continue
		}
		for idForNetwork, cn := range node.ConsensusNodes {
			if signersMap[idForNetwork] {
				continue
			}
			if node.CacheVerify(s.Hash, *sig, cn.Signer.PublicSpendKey) {
				sigs = append(sigs, sig)
				signersMap[idForNetwork] = true
				break
			}
		}
		if n := node.ConsensusPledging; n != nil {
			id := n.Signer.Hash().ForNetwork(node.networkId)
			if id == s.NodeId && s.RoundNumber == 0 && node.CacheVerify(s.Hash, *sig, n.Signer.PublicSpendKey) {
				sigs = append(sigs, sig)
				signersMap[id] = true
			}
		}
		signaturesFilter[sig.String()] = true
	}
	s.Signatures = s.Signatures[:len(sigs)]
	for i := range sigs {
		s.Signatures[i] = sigs[i]
	}
	if !node.verifyFinalization(s.Timestamp, s.Signatures) {
		return nil
	}

	node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash)
	return node.QueueAppendSnapshot(peerId, s, true)
}
