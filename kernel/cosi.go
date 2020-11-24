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
}

type CosiAggregator struct {
	Snapshot    *common.Snapshot
	Transaction *common.VersionedTransaction
	WantTxs     map[crypto.Hash]bool
	Commitments map[int]*crypto.Key
	Responses   map[int]*[32]byte
	committed   map[crypto.Hash]bool
	responsed   map[crypto.Hash]bool
}

type CosiVerifier struct {
	Snapshot *common.Snapshot
	random   *crypto.Key
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
	case CosiActionFinalization:
		return chain.handleFinalization(m)
	}

	return nil
}

func (chain *Chain) cosiCheckSnapshotTimestamp(ts uint64) bool {
	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if ts > uint64(clock.Now().UnixNano())+threshold {
		return false
	}
	if ts+threshold*2 < chain.node.GraphTimestamp {
		return false
	}

	return true
}

func (chain *Chain) cosiSendAnnouncement(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement %v\n", m.Snapshot)
	s := m.Snapshot
	if s.Version != common.SnapshotVersion || s.Signature != nil || s.Timestamp != 0 {
		return nil
	}
	if !chain.node.CheckCatchUpWithPeers() && !(chain.State.FinalRound == nil && s.RoundNumber == 0) {
		logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement CheckCatchUpWithPeers\n")
		return nil
	}

	tx, finalized, err := chain.node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized || tx == nil {
		return nil
	}

	agg := &CosiAggregator{
		Snapshot:    s,
		Transaction: tx,
		WantTxs:     make(map[crypto.Hash]bool),
		Commitments: make(map[int]*crypto.Key),
		Responses:   make(map[int]*[32]byte),
		committed:   make(map[crypto.Hash]bool),
		responsed:   make(map[crypto.Hash]bool),
	}

	if chain.State.FinalRound == nil && s.RoundNumber == 0 && tx.TransactionType() == common.TransactionTypeNodeAccept {
		s.Timestamp = uint64(clock.Now().UnixNano())
		s.Hash = s.PayloadHash()
		v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
		R := v.random.Public()
		chain.CosiVerifiers[s.Hash] = v
		agg.Commitments[len(chain.node.SortedConsensusNodes)] = &R
		chain.CosiAggregators[s.Hash] = agg
		for peerId, _ := range chain.node.ConsensusNodes {
			err := chain.node.Peer.SendSnapshotAnnouncementMessage(peerId, s, R)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if chain.node.ConsensusIndex < 0 || chain.State.FinalRound == nil {
		return nil
	}

	cache := chain.State.CacheRound.Copy()
	final := chain.State.FinalRound.Copy()

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
			link, err := chain.persistStore.ReadLink(cache.NodeId, best.NodeId)
			if err != nil {
				return err
			}
			if best.Number <= link {
				return chain.clearAndQueueSnapshotOrPanic(s)
			}
			cache.References = &common.RoundLink{
				Self:     final.Hash,
				External: best.Hash,
			}
			err = chain.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, cache.References)
			if err != nil {
				panic(err)
			}
			chain.assignNewGraphRound(final, cache)
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
	s.Hash = s.PayloadHash()
	v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
	R := v.random.Public()
	chain.CosiVerifiers[s.Hash] = v
	agg.Commitments[chain.node.ConsensusIndex] = &R
	chain.CosiAggregators[s.Hash] = agg
	for peerId, _ := range chain.node.ConsensusNodes {
		err := chain.node.Peer.SendSnapshotAnnouncementMessage(peerId, m.Snapshot, R)
		if err != nil {
			logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement SendSnapshotAnnouncementMessage(%s, %s) ERROR %s\n", peerId, s.Hash, err.Error())
		}
	}
	return nil
}

func (chain *Chain) cosiHandleAnnouncement(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v\n", m.PeerId, m.Snapshot)
	if chain.node.ConsensusIndex < 0 || !chain.node.CheckCatchUpWithPeers() {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement CheckCatchUpWithPeers\n")
		return nil
	}
	cn := chain.node.getCosensusOrPledgingNode(m.PeerId)
	if cn == nil {
		return nil
	}

	s := m.Snapshot
	if s.Version != common.SnapshotVersion || s.Signature != nil || s.Timestamp == 0 {
		return nil
	}
	if !chain.cosiCheckSnapshotTimestamp(s.Timestamp) {
		return nil
	}

	tx, finalized, err := chain.node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized {
		return nil
	}

	v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
	if chain.State.FinalRound == nil && s.RoundNumber == 0 {
		chain.CosiVerifiers[s.Hash] = v
		err := chain.node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.Hash, v.random.Public(), tx == nil)
		if err != nil {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement SendSnapshotCommitmentMessage(%s, %s) ERROR %s\n", s.NodeId, s.Hash, err.Error())
		}
		return nil
	}

	if !chain.node.ConsensusReady(cn, m.Snapshot.Timestamp) {
		return nil
	}
	if s.RoundNumber == 0 || chain.State.FinalRound == nil {
		return nil
	}

	cache := chain.State.CacheRound.Copy()
	final := chain.State.FinalRound.Copy()

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
		updated, err := chain.updateEmptyHeadRound(m, cache, s)
		if err != nil || !updated {
			return err
		}
		cache.References = &common.RoundLink{
			Self:     s.References.Self,
			External: s.References.External,
		}
		err = chain.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, cache.References)
		if err != nil {
			panic(err)
		}
		chain.assignNewGraphRound(final, cache)
		return chain.queueActionOrPanic(m)
	}
	if s.RoundNumber == cache.Number+1 {
		round, _, err := chain.startNewRound(s, cache, false)
		if err != nil {
			logger.Verbosef("ERROR verifyExternalSnapshot %s %d %s %s\n", s.NodeId, s.RoundNumber, s.Transaction, err.Error())
			return chain.queueActionOrPanic(m)
		} else if round == nil {
			return nil
		} else {
			final = round
		}
		cache = &CacheRound{
			NodeId:     s.NodeId,
			Number:     s.RoundNumber,
			Timestamp:  s.Timestamp,
			References: s.References,
		}
		err = chain.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
	}

	chain.assignNewGraphRound(final, cache)
	if err := cache.ValidateSnapshot(s, false); err != nil {
		return nil
	}

	chain.CosiVerifiers[s.Hash] = v
	err = chain.node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.Hash, v.random.Public(), tx == nil)
	if err != nil {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement SendSnapshotCommitmentMessage(%s, %s) ERROR %s\n", s.NodeId, s.Hash, err.Error())
	}
	return nil
}

func (chain *Chain) cosiHandleCommitment(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment %v\n", m)
	cn := chain.node.ConsensusNodes[m.PeerId]
	if cn == nil {
		return nil
	}

	ann := chain.CosiAggregators[m.SnapshotHash]
	if ann == nil || ann.Snapshot.Hash != m.SnapshotHash {
		return nil
	}
	if ann.committed[m.PeerId] {
		return nil
	}
	if !chain.node.CheckCatchUpWithPeers() && !(chain.State.FinalRound == nil && ann.Snapshot.RoundNumber == 0) {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment CheckCatchUpWithPeers\n")
		return nil
	}
	if !chain.node.ConsensusReady(cn, ann.Snapshot.Timestamp) {
		return nil
	}
	ann.committed[m.PeerId] = true

	base := chain.node.ConsensusThreshold(ann.Snapshot.Timestamp)
	if len(ann.Commitments) >= base {
		return nil
	}
	for i, id := range chain.node.SortedConsensusNodes {
		if id == m.PeerId {
			ann.Commitments[i] = m.Commitment
			ann.WantTxs[m.PeerId] = m.WantTx
			break
		}
	}
	if len(ann.Commitments) < base {
		return nil
	}

	tx, finalized, err := chain.node.checkCacheSnapshotTransaction(ann.Snapshot)
	if err != nil || finalized || tx == nil {
		return nil
	}

	cosi, err := crypto.CosiAggregateCommitment(ann.Commitments)
	if err != nil {
		return err
	}
	ann.Snapshot.Signature = cosi
	v := chain.CosiVerifiers[m.SnapshotHash]
	priv := chain.node.Signer.PrivateSpendKey
	publics := chain.ConsensusKeys(ann.Snapshot.RoundNumber, ann.Snapshot.Timestamp)
	response, err := cosi.Response(&priv, v.random, publics, m.SnapshotHash[:])
	if err != nil {
		return err
	}
	if chain.State.FinalRound == nil && ann.Snapshot.RoundNumber == 0 && tx.TransactionType() == common.TransactionTypeNodeAccept {
		ann.Responses[len(chain.node.SortedConsensusNodes)] = &response
	} else {
		ann.Responses[chain.node.ConsensusIndex] = &response
	}
	copy(cosi.Signature[32:], response[:])
	for id, _ := range chain.node.ConsensusNodes {
		if wantTx, found := ann.WantTxs[id]; !found {
			continue
		} else if wantTx {
			err = chain.node.Peer.SendTransactionChallengeMessage(id, m.SnapshotHash, cosi, tx)
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
	if chain.node.ConsensusIndex < 0 || !chain.node.CheckCatchUpWithPeers() {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleChallenge CheckCatchUpWithPeers\n")
		return nil
	}
	if chain.node.getCosensusOrPledgingNode(m.PeerId) == nil {
		return nil
	}

	v := chain.CosiVerifiers[m.SnapshotHash]
	if v == nil || v.Snapshot.Hash != m.SnapshotHash {
		return nil
	}

	if m.Transaction != nil {
		err := chain.node.CachePutTransaction(m.PeerId, m.Transaction)
		if err != nil {
			return err
		}
	}

	s := v.Snapshot
	if !chain.cosiCheckSnapshotTimestamp(s.Timestamp) {
		return nil
	}

	tx, finalized, err := chain.node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized || tx == nil {
		return nil
	}

	var sig crypto.Signature
	copy(sig[:], s.Commitment[:])
	copy(sig[32:], m.Signature.Signature[32:])
	pub := chain.node.getCosensusOrPledgingNode(s.NodeId).Signer.PublicSpendKey
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
	if chain.node.ConsensusNodes[m.PeerId] == nil {
		return nil
	}

	agg := chain.CosiAggregators[m.SnapshotHash]
	if agg == nil || agg.Snapshot.Hash != m.SnapshotHash {
		return nil
	}
	if agg.responsed[m.PeerId] {
		return nil
	}
	if !chain.node.CheckCatchUpWithPeers() && !(chain.State.FinalRound == nil && agg.Snapshot.RoundNumber == 0) {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse CheckCatchUpWithPeers\n")
		return nil
	}
	agg.responsed[m.PeerId] = true
	if len(agg.Responses) >= len(agg.Commitments) {
		return nil
	}
	base := chain.node.ConsensusThreshold(agg.Snapshot.Timestamp)
	if len(agg.Commitments) < base {
		return nil
	}

	s := agg.Snapshot
	tx, finalized, err := chain.node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized || tx == nil {
		return nil
	}

	for i, id := range chain.node.SortedConsensusNodes {
		if id == m.PeerId {
			agg.Responses[i] = m.Response
			break
		}
	}
	if len(agg.Responses) != len(agg.Commitments) {
		return nil
	}

	index := -1
	for i, id := range chain.node.SortedConsensusNodes {
		if id == m.PeerId {
			index = i
			break
		}
	}
	if index < 0 {
		return nil
	}

	publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	err = s.Signature.VerifyResponse(publics, index, m.Response, m.SnapshotHash[:])
	if err != nil {
		return nil
	}

	s.Signature.AggregateResponse(publics, agg.Responses, m.SnapshotHash[:], false)
	if !chain.node.CacheVerifyCosi(m.SnapshotHash, s.Signature, publics, base) {
		return nil
	}

	if chain.State.FinalRound == nil && s.RoundNumber == 0 && tx.TransactionType() == common.TransactionTypeNodeAccept {
		err := chain.node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
		for id, _ := range chain.node.ConsensusNodes {
			err := chain.node.Peer.SendSnapshotFinalizationMessage(id, s)
			if err != nil {
				logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse SendSnapshotFinalizationMessage(%s, %s) ERROR %s\n", id, m.SnapshotHash, err.Error())
			}
		}
		return chain.node.reloadConsensusNodesList(s, tx)
	}

	cache := chain.State.CacheRound.Copy()
	final := chain.State.FinalRound.Copy()
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

	for id, _ := range chain.node.ConsensusNodes {
		if !agg.responsed[id] {
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
	return chain.node.reloadConsensusNodesList(s, tx)
}

func (chain *Chain) cosiHandleFinalization(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleFinalization %s %v\n", m.PeerId, m.Snapshot)
	s, tx := m.Snapshot, m.Transaction

	if chain.State.FinalRound == nil && s.RoundNumber == 0 && tx.TransactionType() == common.TransactionTypeNodeAccept {
		err := chain.node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
		return chain.node.reloadConsensusNodesList(s, tx)
	}

	cache := chain.State.CacheRound.Copy()
	final := chain.State.FinalRound.Copy()

	if s.RoundNumber < cache.Number {
		logger.Debugf("ERROR cosiHandleFinalization expired round %s %s %d %d\n", m.PeerId, s.Hash, s.RoundNumber, cache.Number)
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return nil
	}
	if s.RoundNumber == cache.Number && !s.References.Equal(cache.References) {
		updated, err := chain.updateEmptyHeadRound(m, cache, s)
		if err != nil || !updated {
			return err
		}
		cache.References = &common.RoundLink{
			Self:     s.References.Self,
			External: s.References.External,
		}
		err = chain.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, cache.References)
		if err != nil {
			panic(err)
		}
		chain.assignNewGraphRound(final, cache)
		return nil
	}
	if s.RoundNumber == cache.Number+1 {
		if round, _, err := chain.startNewRound(s, cache, false); err != nil {
			return nil
		} else if round == nil {
			logger.Verbosef("ERROR cosiHandleFinalization startNewRound empty %s %v\n", m.PeerId, s)
			return nil
		} else {
			final = round
		}
		cache = &CacheRound{
			NodeId:     s.NodeId,
			Number:     s.RoundNumber,
			Timestamp:  s.Timestamp,
			References: s.References,
		}
		err := chain.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
	}

	chain.assignNewGraphRound(final, cache)
	if err := cache.ValidateSnapshot(s, false); err != nil {
		logger.Verbosef("ERROR cosiHandleFinalization ValidateSnapshot %s %v %s\n", m.PeerId, s, err.Error())
		return nil
	}
	chain.node.TopoWrite(s)
	if err := cache.ValidateSnapshot(s, true); err != nil {
		panic("should never be here")
	}
	m.finalized = true
	return chain.node.reloadConsensusNodesList(s, tx)
}

func (chain *Chain) handleFinalization(m *CosiAction) error {
	logger.Debugf("CosiLoop cosiHandleAction handleFinalization %s %v\n", m.PeerId, m.Snapshot)
	s := m.Snapshot
	m.WantTx = false
	if !chain.verifyFinalization(s) {
		logger.Verbosef("ERROR handleFinalization verifyFinalization %s %v %d\n", m.PeerId, s, chain.node.ConsensusThreshold(s.Timestamp))
		return nil
	}

	if cache := chain.State.CacheRound; cache != nil {
		if s.RoundNumber < cache.Number {
			logger.Debugf("ERROR handleFinalization expired round %s %s %d %d\n", m.PeerId, s.Hash, s.RoundNumber, cache.Number)
			return nil
		}
		if s.RoundNumber > cache.Number+1 {
			return nil
		}
	}

	dummy, err := chain.tryToStartNewRound(s)
	if err != nil {
		logger.Verbosef("ERROR handleFinalization tryToStartNewRound %s %s %d %s\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp), err.Error())
		return nil
	} else if dummy {
		logger.Verbosef("ERROR handleFinalization tryToStartNewRound DUMMY %s %s %d\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp))
		return nil
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
	if s.RoundNumber == 0 && tx.TransactionType() != common.TransactionTypeNodeAccept {
		return fmt.Errorf("invalid initial transaction type %d", tx.TransactionType())
	}

	m.Transaction = tx
	return chain.cosiHandleFinalization(m)
}

func (node *Node) CosiQueueExternalAnnouncement(peerId crypto.Hash, s *common.Snapshot, commitment *crypto.Key) error {
	if node.getCosensusOrPledgingNode(peerId) == nil {
		return nil
	}
	chain := node.GetOrCreateChain(s.NodeId)

	if s.Version != common.SnapshotVersion {
		return nil
	}
	if s.NodeId == node.IdForNetwork || s.NodeId != peerId {
		return nil
	}
	if s.Signature != nil || s.Timestamp == 0 || commitment == nil {
		return nil
	}
	s.Hash = s.PayloadHash()
	s.Commitment = commitment

	m := &CosiAction{
		PeerId:   peerId,
		Action:   CosiActionExternalAnnouncement,
		Snapshot: s,
	}
	chain.AppendCosiAction(m)
	return nil
}

func (node *Node) CosiAggregateSelfCommitments(peerId crypto.Hash, snap crypto.Hash, commitment *crypto.Key, wantTx bool) error {
	if node.ConsensusNodes[peerId] == nil {
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
	if node.getCosensusOrPledgingNode(peerId) == nil {
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
	if node.ConsensusNodes[peerId] == nil {
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
	if node.custom.Node.ConsensusOnly && node.getCosensusOrPledgingNode(peerId) == nil {
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
	if chain == nil {
		return nil
	}

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

func (node *Node) getCosensusOrPledgingNode(peerId crypto.Hash) *CNode {
	if n := node.ConsensusPledging; n != nil && n.IdForNetwork == peerId {
		return n
	}
	return node.ConsensusNodes[peerId]
}
