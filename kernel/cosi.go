package kernel

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
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
	finalized    bool
	data         *CosiChainData
}

type CosiChainData struct {
	PN *CNode
	CN *CNode
	TX *common.VersionedTransaction
	F  bool
}

type CosiAggregator struct {
	Snapshot    *common.Snapshot
	Transaction *common.VersionedTransaction
	WantTxs     map[crypto.Hash]bool
	Commitments map[int]*crypto.Key
	Responses   map[int]*[32]byte
}

type CosiVerifier struct {
	Snapshot   *common.Snapshot
	Commitment *crypto.Key
	random     *crypto.Key
}

func (chain *Chain) cosiHook(m *CosiAction) (bool, error) {
	if !chain.running {
		return false, nil
	}
	err := chain.cosiHandleAction(m)
	if err != nil {
		return false, err
	}
	if m.Action != CosiActionFinalization {
		return false, nil
	}
	if m.finalized || !m.WantTx || m.PeerId == chain.node.IdForNetwork {
		return m.finalized, nil
	}
	logger.Debugf("cosiHook finalized snapshot without transaction %s %s %s\n", m.PeerId, m.SnapshotHash, m.Snapshot.Transaction)
	chain.node.Peer.SendTransactionRequestMessage(m.PeerId, m.Snapshot.Transaction)
	return m.finalized, nil
}

func (chain *Chain) cosiHandleAction(m *CosiAction) error {
	if m.Action == CosiActionFinalization {
		return chain.cosiHandleFinalization(m)
	}
	if err := chain.checkActionSanity(m); err != nil {
		logger.Debugf("checkActionSanity %d ERROR %s\n", m.Action, err)
		return nil
	}

	switch m.Action {
	case CosiActionSelfEmpty:
		return chain.cosiSendAnnouncement(m)
	case CosiActionSelfCommitment:
		return chain.cosiHandleCommitment(m)
	case CosiActionSelfResponse:
		return chain.cosiHandleResponse(m)
	case CosiActionExternalAnnouncement:
		return chain.cosiHandleAnnouncement(m)
	case CosiActionExternalChallenge:
		return chain.cosiHandleChallenge(m)
	}

	return nil
}

func (chain *Chain) checkActionSanity(m *CosiAction) error {
	s := m.Snapshot
	switch m.Action {
	case CosiActionSelfEmpty:
		if chain.ChainId != chain.node.IdForNetwork {
			return fmt.Errorf("self action announcement chain %s %s", chain.ChainId, chain.node.IdForNetwork)
		}
		if chain.ChainId != m.PeerId {
			return fmt.Errorf("self action announcement peer %s %s", chain.ChainId, m.PeerId)
		}
		if s.Signature != nil || s.Timestamp != 0 {
			return fmt.Errorf("only empty snapshot can be announced")
		}
	case CosiActionSelfCommitment, CosiActionSelfResponse:
		if chain.ChainId != chain.node.IdForNetwork {
			return fmt.Errorf("self action aggregation chain %s %s", chain.ChainId, chain.node.IdForNetwork)
		}
		if chain.ChainId == m.PeerId {
			return fmt.Errorf("self action aggregation peer %s %s", chain.ChainId, m.PeerId)
		}
		if a := chain.CosiAggregators[m.SnapshotHash]; a != nil {
			s = a.Snapshot
		}
	case CosiActionExternalAnnouncement:
		if chain.ChainId == chain.node.IdForNetwork {
			return fmt.Errorf("external action announcement chain %s %s", chain.ChainId, chain.node.IdForNetwork)
		}
		if chain.ChainId != m.PeerId {
			return fmt.Errorf("external action announcement peer %s %s", chain.ChainId, m.PeerId)
		}
		if s.Signature != nil || s.Timestamp == 0 {
			return fmt.Errorf("only empty snapshot with timestamp can be announced")
		}
	case CosiActionExternalChallenge:
		if chain.ChainId == chain.node.IdForNetwork {
			return fmt.Errorf("external action challenge chain %s %s", chain.ChainId, chain.node.IdForNetwork)
		}
		if chain.ChainId != m.PeerId {
			return fmt.Errorf("external action challenge peer %s %s", chain.ChainId, m.PeerId)
		}
		if v := chain.CosiVerifiers[m.SnapshotHash]; v != nil {
			s = v.Snapshot
		}
	}

	if s == nil {
		return fmt.Errorf("no snapshot in cosi")
	}
	if s.Version != common.SnapshotVersion {
		return fmt.Errorf("invalid snapshot version %d", s.Version)
	}
	if s.NodeId != chain.ChainId {
		return fmt.Errorf("invalid snapshot node id %s %s", s.NodeId, chain.ChainId)
	}

	if m.Transaction != nil {
		err := chain.node.CachePutTransaction(m.PeerId, m.Transaction)
		if err != nil {
			return err
		}
	}

	if m.Action != CosiActionSelfEmpty {
		if m.SnapshotHash != s.Hash {
			return fmt.Errorf("invalid snapshot hash %s %s", m.SnapshotHash, s.Hash)
		}
		threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
		if s.Timestamp > uint64(clock.Now().UnixNano())+threshold {
			return fmt.Errorf("future snapshot timestamp %d", s.Timestamp)
		}
		if s.Timestamp+threshold*2 < chain.node.GraphTimestamp {
			return fmt.Errorf("past snapshot timestamp %d", s.Timestamp)
		}
		if !chain.IsPledging() && !chain.node.CheckCatchUpWithPeers() {
			return fmt.Errorf("node is slow in catching up")
		}
	}

	cn := chain.node.GetAcceptedOrPledgingNode(chain.ChainId)
	if cn == nil {
		return fmt.Errorf("chain node %s not found", chain.ChainId)
	}
	pn := chain.node.GetAcceptedOrPledgingNode(m.PeerId)
	if pn == nil {
		return fmt.Errorf("peer node %s not found", m.PeerId)
	}
	if s.RoundNumber != 0 && !chain.node.ConsensusReady(cn, s.Timestamp) {
		return fmt.Errorf("chain node %s not accepted", cn.IdForNetwork)
	}
	if s.RoundNumber != 0 && !chain.node.ConsensusReady(pn, s.Timestamp) {
		return fmt.Errorf("peer node %s not accepted", pn.IdForNetwork)
	}

	tx, finalized, err := chain.node.validateSnapshotTransaction(s, false)
	if err != nil || finalized {
		return fmt.Errorf("cosi snapshot transaction error %v or finalized %v", err, finalized)
	}
	if m.Action != CosiActionExternalAnnouncement && tx == nil {
		return fmt.Errorf("no transaction found")
	}

	m.data = &CosiChainData{PN: pn, CN: cn, TX: tx, F: finalized}
	return nil
}

func (chain *Chain) cosiSendAnnouncement(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement %v\n", m.Snapshot)
	s, cd := m.Snapshot, m.data
	s.Timestamp = uint64(clock.Now().UnixNano())
	if chain.IsPledging() && s.RoundNumber == 0 && cd.TX.TransactionType() == common.TransactionTypeNodeAccept {
	} else if chain.State.FinalRound == nil {
		return nil
	} else {
		cache, final := chain.StateCopy()
		if len(cache.Snapshots) == 0 && !chain.node.CheckBroadcastedToPeers() {
			return chain.clearAndQueueSnapshotOrPanic(s)
		}
		for {
			s.Timestamp = uint64(clock.Now().UnixNano())
			if s.Timestamp > cache.Timestamp {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if len(cache.Snapshots) == 0 {
			external, err := chain.persistStore.ReadRound(cache.References.External)
			if err != nil {
				return err
			}
			best, _ := chain.determinBestRound(s.Timestamp, chain.ChainId)
			threshold := external.Timestamp + config.SnapshotReferenceThreshold*config.SnapshotRoundGap*36
			if best != nil && best.NodeId != final.NodeId && threshold < best.Start {
				logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement new best external %s:%d:%d => %s:%d:%d\n", external.NodeId, external.Number, external.Timestamp, best.NodeId, best.Number, best.Start)
				references := &common.RoundLink{Self: final.Hash, External: best.Hash}
				updated, err := chain.updateEmptyHeadRoundAndPersist(m, cache, references)
				if err != nil {
					return err
				}
				if updated {
					chain.assignNewGraphRound(final, cache)
				}
				return chain.clearAndQueueSnapshotOrPanic(s)
			}
		} else if start, _ := cache.Gap(); s.Timestamp >= start+config.SnapshotRoundGap {
			best, _ := chain.determinBestRound(s.Timestamp, chain.ChainId)
			if best == nil {
				logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement no best available\n")
				return chain.clearAndQueueSnapshotOrPanic(s)
			}
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
			chain.assignNewGraphRound(final, cache)
			err := chain.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
			if err != nil {
				panic(err)
			}
		}
		cache.Timestamp = s.Timestamp

		if len(cache.Snapshots) > 0 && s.Timestamp > cache.Snapshots[0].Timestamp+uint64(config.SnapshotRoundGap*4/5) {
			return chain.clearAndQueueSnapshotOrPanic(s)
		}

		s.RoundNumber = cache.Number
		s.References = cache.References
	}

	s.Hash = s.PayloadHash()
	agg := &CosiAggregator{
		Snapshot:    s,
		Transaction: cd.TX,
		WantTxs:     make(map[crypto.Hash]bool),
		Commitments: make(map[int]*crypto.Key),
		Responses:   make(map[int]*[32]byte),
	}
	v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
	R := v.random.Public()
	chain.CosiVerifiers[s.Hash] = v
	agg.Commitments[cd.CN.ConsensusIndex] = &R
	chain.CosiAggregators[s.Hash] = agg
	nodes := chain.node.NodesListWithoutState(s.Timestamp, true)
	for _, cn := range nodes {
		peerId := cn.IdForNetwork
		err := chain.node.Peer.SendSnapshotAnnouncementMessage(peerId, m.Snapshot, R)
		if err != nil {
			logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement SendSnapshotAnnouncementMessage(%s, %s) ERROR %s\n", peerId, s.Hash, err.Error())
		}
	}
	return nil
}

func (chain *Chain) cosiHandleAnnouncement(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v\n", m.PeerId, m.Snapshot)

	s, cd := m.Snapshot, m.data
	if chain.IsPledging() && s.RoundNumber == 0 {
	} else if chain.State.FinalRound == nil {
		return nil
	} else {
		cache, final := chain.StateCopy()
		if s.RoundNumber < cache.Number {
			return nil
		}
		if s.RoundNumber > cache.Number+1 {
			return chain.queueActionOrPanic(m)
		}
		if s.Timestamp <= final.Start+config.SnapshotRoundGap {
			return nil
		}
		if s.RoundNumber == cache.Number && !s.References.Equal(cache.References) {
			updated, err := chain.updateEmptyHeadRoundAndPersist(m, cache, s.References)
			if err != nil || !updated {
				return err
			}
			chain.assignNewGraphRound(final, cache)
			return chain.queueActionOrPanic(m)
		}
		if s.RoundNumber == cache.Number+1 {
			nc, nf, _, err := chain.startNewRoundAndPersist(s, cache, false)
			if err != nil {
				return chain.queueActionOrPanic(m)
			} else if nf == nil {
				return nil
			}
			cache, final = nc, nf
		}

		chain.assignNewGraphRound(final, cache)
		if err := cache.ValidateSnapshot(s, false); err != nil {
			return nil
		}
	}

	r := crypto.CosiCommit(rand.Reader)
	v := &CosiVerifier{Snapshot: s, Commitment: m.Commitment, random: r}
	chain.CosiVerifiers[s.Hash] = v
	err := chain.node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.Hash, r.Public(), cd.TX == nil)
	if err != nil {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement SendSnapshotCommitmentMessage(%s, %s) ERROR %s\n", s.NodeId, s.Hash, err.Error())
	}
	return nil
}

func (chain *Chain) cosiHandleCommitment(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment %v\n", m)

	ann := chain.CosiAggregators[m.SnapshotHash]
	s, cd := ann.Snapshot, m.data
	if ann.Commitments[cd.PN.ConsensusIndex] != nil {
		return nil
	}
	base := chain.node.ConsensusThreshold(ann.Snapshot.Timestamp)
	if len(ann.Commitments) >= base {
		return nil
	}
	ann.Commitments[cd.PN.ConsensusIndex] = m.Commitment
	ann.WantTxs[m.PeerId] = m.WantTx
	if len(ann.Commitments) < base {
		return nil
	}

	cosi, err := crypto.CosiAggregateCommitment(ann.Commitments)
	if err != nil {
		return err
	}
	s.Signature = cosi
	v := chain.CosiVerifiers[m.SnapshotHash]
	priv := chain.node.Signer.PrivateSpendKey
	publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	response, err := cosi.Response(&priv, v.random, publics, m.SnapshotHash[:])
	if err != nil {
		return err
	}
	ann.Responses[cd.CN.ConsensusIndex] = &response
	copy(cosi.Signature[32:], response[:])

	nodes := chain.node.NodesListWithoutState(s.Timestamp, true)
	for _, cn := range nodes {
		id := cn.IdForNetwork
		if wantTx, found := ann.WantTxs[id]; !found {
			continue
		} else if wantTx {
			err = chain.node.Peer.SendTransactionChallengeMessage(id, m.SnapshotHash, cosi, cd.TX)
		} else {
			err = chain.node.Peer.SendTransactionChallengeMessage(id, m.SnapshotHash, cosi, nil)
		}
		if err != nil {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment SendTransactionChallengeMessage(%s, %s) ERROR %s\n", id, m.SnapshotHash, err.Error())
		}
	}
	return nil
}

func (chain *Chain) cosiHandleChallenge(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleChallenge %v\n", m)
	v := chain.CosiVerifiers[m.SnapshotHash]
	s, cd := v.Snapshot, m.data

	var sig crypto.Signature
	copy(sig[:], v.Commitment[:])
	copy(sig[32:], m.Signature.Signature[32:])
	pub := cd.CN.Signer.PublicSpendKey
	publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	challenge, err := m.Signature.Challenge(publics, m.SnapshotHash[:])
	if err != nil {
		return nil
	}
	if !pub.VerifyWithChallenge(m.SnapshotHash[:], sig, challenge) {
		return nil
	}

	priv := chain.node.Signer.PrivateSpendKey
	response, err := m.Signature.Response(&priv, v.random, publics, m.SnapshotHash[:])
	if err != nil {
		return err
	}
	err = chain.node.Peer.SendSnapshotResponseMessage(m.PeerId, m.SnapshotHash, response)
	if err != nil {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleChallenge SendSnapshotResponseMessage(%s, %s) ERROR %s\n", m.PeerId, m.SnapshotHash, err.Error())
	}
	return nil
}

func (chain *Chain) cosiHandleResponse(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v\n", m)
	agg := chain.CosiAggregators[m.SnapshotHash]
	s, cd := agg.Snapshot, m.data
	if agg.Responses[cd.PN.ConsensusIndex] != nil {
		return nil
	}
	if len(agg.Responses) >= len(agg.Commitments) {
		return nil
	}
	agg.Responses[cd.PN.ConsensusIndex] = m.Response
	if len(agg.Responses) != len(agg.Commitments) {
		return nil
	}

	publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	err := s.Signature.VerifyResponse(publics, cd.PN.ConsensusIndex, m.Response, m.SnapshotHash[:])
	if err != nil {
		return nil
	}

	base := chain.node.ConsensusThreshold(agg.Snapshot.Timestamp)
	s.Signature.AggregateResponse(publics, agg.Responses, m.SnapshotHash[:], false)
	if !chain.node.CacheVerifyCosi(m.SnapshotHash, s.Signature, publics, base) {
		return nil
	}

	if chain.IsPledging() && s.RoundNumber == 0 && cd.TX.TransactionType() == common.TransactionTypeNodeAccept {
		err := chain.node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
	} else {
		cache, final := chain.StateCopy()
		if s.RoundNumber > cache.Number {
			panic(fmt.Sprintf("should never be here %d %d", cache.Number, s.RoundNumber))
		}
		if s.RoundNumber < cache.Number {
			return chain.clearAndQueueSnapshotOrPanic(s)
		}
		if !s.References.Equal(cache.References) {
			return chain.clearAndQueueSnapshotOrPanic(s)
		}
		if err := cache.ValidateSnapshot(s, false); err != nil {
			return chain.clearAndQueueSnapshotOrPanic(s)
		}

		chain.node.TopoWrite(s)
		if err := cache.ValidateSnapshot(s, true); err != nil {
			panic("should never be here")
		}
		chain.assignNewGraphRound(final, cache)
	}

	nodes := chain.node.NodesListWithoutState(s.Timestamp, true)
	for _, cn := range nodes {
		id := cn.IdForNetwork
		if agg.Responses[cn.ConsensusIndex] == nil {
			err := chain.node.SendTransactionToPeer(id, agg.Snapshot.Transaction)
			if err != nil {
				logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse SendTransactionToPeer(%s, %s) ERROR %s\n", id, m.SnapshotHash, err.Error())
			}
		}
		err := chain.node.Peer.SendSnapshotFinalizationMessage(id, agg.Snapshot)
		if err != nil {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse SendSnapshotFinalizationMessage(%s, %s) ERROR %s\n", id, m.SnapshotHash, err.Error())
		}
	}
	return chain.node.reloadConsensusNodesList(s, cd.TX)
}

func (chain *Chain) cosiHandleFinalization(m *CosiAction) error {
	logger.Debugf("CosiLoop cosiHandleAction handleFinalization %s %v\n", m.PeerId, m.Snapshot)
	s := m.Snapshot
	m.WantTx = false
	if !chain.verifyFinalization(s) {
		logger.Verbosef("ERROR handleFinalization verifyFinalization %s %v %d\n", m.PeerId, s, chain.node.ConsensusThreshold(s.Timestamp))
		return nil
	}

	if cache := chain.State.CacheRound; cache != nil {
		if s.RoundNumber < cache.Number {
			logger.Debugf("ERROR cosiHandleFinalization expired round %s %s %d %d\n", m.PeerId, s.Hash, s.RoundNumber, cache.Number)
			return nil
		}
		if s.RoundNumber > cache.Number+1 {
			return nil
		}
		if s.RoundNumber == cache.Number+1 {
			nc, nf, dummy, err := chain.startNewRoundAndPersist(s, cache, true)
			if err != nil || nf == nil {
				logger.Verbosef("ERROR cosiHandleFinalization startNewRound %s %v\n", m.PeerId, s)
				return nil
			}
			chain.assignNewGraphRound(nf, nc)
			if dummy {
				logger.Verbosef("ERROR handleFinalization startNewRound DUMMY %s %s %d\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp))
				return nil
			}
		}
	}

	tx, inNode, err := chain.node.checkFinalSnapshotTransaction(s)
	if err != nil {
		logger.Verbosef("ERROR handleFinalization checkFinalSnapshotTransaction %s %s %d %s\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp), err.Error())
		return nil
	} else if inNode {
		m.finalized = true
		return nil
	} else if tx == nil {
		logger.Verbosef("ERROR handleFinalization checkFinalSnapshotTransaction %s %s %d %s\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp), "tx empty")
		m.WantTx = true
		return nil
	}

	if chain.IsPledging() && s.RoundNumber == 0 && tx.TransactionType() == common.TransactionTypeNodeAccept {
		err := chain.node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
		return chain.node.reloadConsensusNodesList(s, tx)
	}
	if chain.State.FinalRound == nil {
		return nil
	}
	cache, final := chain.StateCopy()
	if s.RoundNumber != cache.Number {
		return nil
	}

	if !s.References.Equal(cache.References) {
		updated, err := chain.updateEmptyHeadRoundAndPersist(m, cache, s.References)
		if err != nil || !updated {
			return err
		}
		chain.assignNewGraphRound(final, cache)
		return nil
	}

	if err := cache.ValidateSnapshot(s, false); err != nil {
		logger.Verbosef("ERROR cosiHandleFinalization ValidateSnapshot %s %v %s\n", m.PeerId, s, err.Error())
		return nil
	}
	chain.node.TopoWrite(s)
	if err := cache.ValidateSnapshot(s, true); err != nil {
		panic("should never be here")
	}
	m.finalized = true
	chain.assignNewGraphRound(final, cache)
	return chain.node.reloadConsensusNodesList(s, tx)
}

func (node *Node) CosiQueueExternalAnnouncement(peerId crypto.Hash, s *common.Snapshot, commitment *crypto.Key) error {
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		return nil
	}
	chain := node.GetOrCreateChain(s.NodeId)

	s.Hash = s.PayloadHash()
	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionExternalAnnouncement,
		Snapshot:     s,
		Commitment:   commitment,
		SnapshotHash: s.Hash,
	}
	chain.AppendCosiAction(m)
	return nil
}

func (node *Node) CosiAggregateSelfCommitments(peerId crypto.Hash, snap crypto.Hash, commitment *crypto.Key, wantTx bool) error {
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		return nil
	}
	chain := node.GetOrCreateChain(node.IdForNetwork)

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionSelfCommitment,
		SnapshotHash: snap,
		Commitment:   commitment,
		WantTx:       wantTx,
	}
	chain.AppendCosiAction(m)
	return nil
}

func (node *Node) CosiQueueExternalChallenge(peerId crypto.Hash, snap crypto.Hash, cosi *crypto.CosiSignature, ver *common.VersionedTransaction) error {
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		return nil
	}
	chain := node.GetOrCreateChain(peerId)

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionExternalChallenge,
		SnapshotHash: snap,
		Signature:    cosi,
		Transaction:  ver,
	}
	chain.AppendCosiAction(m)
	return nil
}

func (node *Node) CosiAggregateSelfResponses(peerId crypto.Hash, snap crypto.Hash, response *[32]byte) error {
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		return nil
	}
	chain := node.GetOrCreateChain(node.IdForNetwork)

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionSelfResponse,
		SnapshotHash: snap,
		Response:     response,
	}
	chain.AppendCosiAction(m)
	return nil
}

func (node *Node) VerifyAndQueueAppendSnapshotFinalization(peerId crypto.Hash, s *common.Snapshot) error {
	s.Hash = s.PayloadHash()
	logger.Debugf("VerifyAndQueueAppendSnapshotFinalization(%s, %s)\n", peerId, s.Hash)
	if node.custom.Node.ConsensusOnly && node.GetAcceptedOrPledgingNode(peerId) == nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) invalid consensus peer\n", peerId, s.Hash)
		return nil
	}

	node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash)
	err := node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash)
	if err != nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) SendSnapshotConfirmMessage error %s\n", peerId, s.Hash, err)
		return nil
	}
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) already finalized %t %v\n", peerId, s.Hash, inNode, err)
		return err
	}

	hasTx, err := node.checkTxInStorage(s.Transaction)
	if err != nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) check tx error %s\n", peerId, s.Hash, err)
	} else if !hasTx {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) SendTransactionRequestMessage %s\n", peerId, s.Hash, s.Transaction)
		node.Peer.SendTransactionRequestMessage(peerId, s.Transaction)
	}

	chain := node.GetOrCreateChain(s.NodeId)

	if s.Version == 0 {
		err := chain.legacyAppendFinalization(peerId, s)
		if err != nil {
			logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) legacyAppendFinalization error %s\n", peerId, s.Hash, err)
		}
		return err
	}
	if !chain.verifyFinalization(s) {
		logger.Verbosef("ERROR VerifyAndQueueAppendSnapshotFinalization %s %v %d\n", peerId, s, node.ConsensusThreshold(s.Timestamp))
		return nil
	}

	err = chain.AppendFinalSnapshot(peerId, s)
	if err != nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) chain error %s\n", peerId, s.Hash, err)
	}
	return err
}
