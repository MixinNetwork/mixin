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
	"github.com/MixinNetwork/mixin/util"
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

	key crypto.Hash
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

func (chain *Chain) CosiLoop() error {
	timer := util.NewTimer(time.Second)
	defer timer.Stop()

	for {
		select {
		case <-chain.node.done:
			return nil
		case m := <-chain.cosiActionsChan:
			err := chain.cosiHandleAction(m, timer)
			if err != nil {
				return err
			}
		}
	}
}

func (chain *Chain) cosiHandleAction(m *CosiAction, timer *util.Timer) error {
	defer chain.node.UpdateFinalCache()

	switch m.Action {
	case CosiActionSelfEmpty:
		return chain.cosiSendAnnouncement(m, timer)
	case CosiActionSelfCommitment:
		return chain.cosiHandleCommitment(m, timer)
	case CosiActionSelfResponse:
		return chain.cosiHandleResponse(m, timer)
	case CosiActionExternalAnnouncement:
		return chain.cosiHandleAnnouncement(m, timer)
	case CosiActionExternalChallenge:
		return chain.cosiHandleChallenge(m, timer)
	case CosiActionFinalization:
		return chain.handleFinalization(m)
	}

	return nil
}

func (chain *Chain) cosiSendAnnouncement(m *CosiAction, timer *util.Timer) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement %v\n", m.Snapshot)
	s := m.Snapshot
	if chain.ChainId != chain.node.IdForNetwork || s.NodeId != chain.ChainId || s.NodeId != m.PeerId {
		panic(fmt.Errorf("should never be here %s %s %s %s", chain.node.IdForNetwork, chain.ChainId, s.NodeId, m.PeerId))
	}
	if s.Version != common.SnapshotVersion || s.Signature != nil || s.Timestamp != 0 {
		return nil
	}
	if !chain.node.CheckCatchUpWithPeers() && !chain.node.checkInitialAcceptSnapshotWeak(m.Snapshot) {
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

	if chain.node.checkInitialAcceptSnapshot(s, tx) {
		s.Timestamp = uint64(clock.Now().UnixNano())
		s.Hash = s.PayloadHash()
		v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
		R := v.random.Public()
		chain.CosiVerifiers[s.Hash] = v
		agg.Commitments[len(chain.node.SortedConsensusNodes)] = &R
		chain.CosiAggregators[s.Hash] = agg
		for peerId, _ := range chain.node.ConsensusNodes {
			err := chain.node.Peer.SendSnapshotAnnouncementMessage(peerId, s, R, timer)
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
		best := chain.node.determinBestRound(s.NodeId, s.Timestamp)
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
		best := chain.node.determinBestRound(s.NodeId, s.Timestamp)
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
	chain.assignNewGraphRound(final, cache)
	chain.CosiAggregators[s.Hash] = agg
	for peerId, _ := range chain.node.ConsensusNodes {
		err := chain.node.Peer.SendSnapshotAnnouncementMessage(peerId, m.Snapshot, R, timer)
		if err != nil {
			return err
		}
	}
	return nil
}

func (chain *Chain) cosiHandleAnnouncement(m *CosiAction, timer *util.Timer) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v\n", m.PeerId, m.Snapshot)
	if chain.node.ConsensusIndex < 0 || !chain.node.CheckCatchUpWithPeers() {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement CheckCatchUpWithPeers\n")
		return nil
	}
	cn := chain.node.getPeerConsensusNode(m.PeerId)
	if cn == nil {
		return nil
	}
	if cn.Timestamp+uint64(config.KernelNodeAcceptPeriodMinimum) >= m.Snapshot.Timestamp && !chain.node.genesisNodesMap[cn.IdForNetwork(chain.node.networkId)] {
		return nil
	}

	s := m.Snapshot
	if chain.ChainId == chain.node.IdForNetwork || s.NodeId == chain.node.IdForNetwork || s.NodeId != m.PeerId {
		panic(fmt.Errorf("should never be here %s %s %s", chain.node.IdForNetwork, s.NodeId, s.Signature))
	}
	if s.Version != common.SnapshotVersion || s.Signature != nil || s.Timestamp == 0 {
		return nil
	}
	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if s.Timestamp > uint64(clock.Now().UnixNano())+threshold {
		return nil
	}
	if s.Timestamp+threshold*2 < chain.node.GraphTimestamp {
		return nil
	}

	tx, finalized, err := chain.node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized {
		return nil
	}

	v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
	if chain.node.checkInitialAcceptSnapshotWeak(s) {
		chain.CosiVerifiers[s.Hash] = v
		return chain.node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.Hash, v.random.Public(), tx == nil, timer)
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
		return chain.queueSnapshotOrPanic(m.PeerId, s)
	}
	if s.Timestamp <= final.Start+config.SnapshotRoundGap {
		return nil
	}
	if s.RoundNumber == cache.Number && !s.References.Equal(cache.References) {
		if len(cache.Snapshots) > 0 {
			return nil
		}
		if s.References.Self != cache.References.Self {
			return nil
		}
		external, err := chain.persistStore.ReadRound(s.References.External)
		if err != nil || external == nil {
			return err
		}
		link, err := chain.persistStore.ReadLink(cache.NodeId, external.NodeId)
		if err != nil {
			return err
		}
		if external.Number < link {
			return nil
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
		return chain.queueSnapshotOrPanic(m.PeerId, s)
	}
	if s.RoundNumber == cache.Number+1 {
		round, _, err := chain.startNewRound(s, cache, false)
		if err != nil {
			logger.Verbosef("ERROR verifyExternalSnapshot %s %d %s %s\n", s.NodeId, s.RoundNumber, s.Transaction, err.Error())
			return chain.queueSnapshotOrPanic(m.PeerId, s)
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
	return chain.node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.Hash, v.random.Public(), tx == nil, timer)
}

func (chain *Chain) cosiHandleCommitment(m *CosiAction, timer *util.Timer) error {
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
	if !chain.node.CheckCatchUpWithPeers() && !chain.node.checkInitialAcceptSnapshotWeak(ann.Snapshot) {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment CheckCatchUpWithPeers\n")
		return nil
	}
	if cn.Timestamp+uint64(config.KernelNodeAcceptPeriodMinimum) >= ann.Snapshot.Timestamp && !chain.node.genesisNodesMap[cn.IdForNetwork(chain.node.networkId)] {
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
	publics := chain.node.ConsensusKeys(ann.Snapshot.Timestamp)
	if chain.node.checkInitialAcceptSnapshot(ann.Snapshot, tx) {
		publics = append(publics, &chain.node.ConsensusPledging.Signer.PublicSpendKey)
	}
	response, err := cosi.Response(&priv, v.random, publics, m.SnapshotHash[:])
	if err != nil {
		return err
	}
	if chain.node.checkInitialAcceptSnapshot(ann.Snapshot, tx) {
		ann.Responses[len(chain.node.SortedConsensusNodes)] = &response
	} else {
		ann.Responses[chain.node.ConsensusIndex] = &response
	}
	copy(cosi.Signature[32:], response[:])
	for id, _ := range chain.node.ConsensusNodes {
		if wantTx, found := ann.WantTxs[id]; !found {
			continue
		} else if wantTx {
			err = chain.node.Peer.SendTransactionChallengeMessage(id, m.SnapshotHash, cosi, tx, timer)
		} else {
			err = chain.node.Peer.SendTransactionChallengeMessage(id, m.SnapshotHash, cosi, nil, timer)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (chain *Chain) cosiHandleChallenge(m *CosiAction, timer *util.Timer) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleChallenge %v\n", m)
	if chain.node.ConsensusIndex < 0 || !chain.node.CheckCatchUpWithPeers() {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleChallenge CheckCatchUpWithPeers\n")
		return nil
	}
	if chain.node.getPeerConsensusNode(m.PeerId) == nil {
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
	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if s.Timestamp > uint64(clock.Now().UnixNano())+threshold {
		return nil
	}
	if s.Timestamp+threshold*2 < chain.node.GraphTimestamp {
		return nil
	}

	tx, finalized, err := chain.node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized || tx == nil {
		return nil
	}

	var sig crypto.Signature
	copy(sig[:], s.Commitment[:])
	copy(sig[32:], m.Signature.Signature[32:])
	pub := chain.node.getPeerConsensusNode(s.NodeId).Signer.PublicSpendKey
	publics := chain.node.ConsensusKeys(s.Timestamp)
	if chain.node.checkInitialAcceptSnapshot(s, tx) {
		publics = append(publics, &chain.node.ConsensusPledging.Signer.PublicSpendKey)
	}
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
	return chain.node.Peer.SendSnapshotResponseMessage(m.PeerId, m.SnapshotHash, response, timer)
}

func (chain *Chain) cosiHandleResponse(m *CosiAction, timer *util.Timer) error {
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
	if !chain.node.CheckCatchUpWithPeers() && !chain.node.checkInitialAcceptSnapshotWeak(agg.Snapshot) {
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

	publics := chain.node.ConsensusKeys(s.Timestamp)
	if chain.node.checkInitialAcceptSnapshot(s, tx) {
		publics = append(publics, &chain.node.ConsensusPledging.Signer.PublicSpendKey)
	}

	err = s.Signature.VerifyResponse(publics, index, m.Response, m.SnapshotHash[:])
	if err != nil {
		return nil
	}

	s.Signature.AggregateResponse(publics, agg.Responses, m.SnapshotHash[:], false)
	if !chain.node.CacheVerifyCosi(m.SnapshotHash, s.Signature, publics, base) {
		return nil
	}

	if chain.node.checkInitialAcceptSnapshot(s, tx) {
		err := chain.node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
		for id, _ := range chain.node.ConsensusNodes {
			err := chain.node.Peer.SendSnapshotFinalizationMessage(id, s, timer)
			if err != nil {
				return err
			}
		}
		return chain.node.reloadConsensusNodesList(s, tx)
	}

	cache := chain.State.CacheRound.Copy()
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

	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: chain.node.TopoCounter.Next(),
	}
	for {
		err := chain.persistStore.WriteSnapshot(topo)
		if err != nil {
			logger.Debugf("ERROR cosiHandleResponse WriteSnapshot %s %s %s\n", m.PeerId, m.SnapshotHash, err.Error())
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
	}
	if err := cache.ValidateSnapshot(s, true); err != nil {
		panic("should never be here")
	}
	chain.State.CacheRound = cache

	for id, _ := range chain.node.ConsensusNodes {
		if !agg.responsed[id] {
			err := chain.node.SendTransactionToPeer(id, agg.Snapshot.Transaction, timer)
			if err != nil {
				return err
			}
		}
		err := chain.node.Peer.SendSnapshotFinalizationMessage(id, agg.Snapshot, timer)
		if err != nil {
			return err
		}
	}
	return chain.node.reloadConsensusNodesList(s, tx)
}

func (chain *Chain) cosiHandleFinalization(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleFinalization %s %v\n", m.PeerId, m.Snapshot)
	s, tx := m.Snapshot, m.Transaction

	if chain.node.checkInitialAcceptSnapshot(s, tx) {
		err := chain.node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
		return chain.node.reloadConsensusNodesList(s, tx)
	}

	cache := chain.State.CacheRound.Copy()
	final := chain.State.FinalRound.Copy()

	if s.RoundNumber < cache.Number {
		logger.Verbosef("ERROR cosiHandleFinalization expired round %s %s %d %d\n", m.PeerId, s.Hash, s.RoundNumber, cache.Number)
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return chain.QueueAppendSnapshot(m.PeerId, s, true)
	}
	if s.RoundNumber == cache.Number && !s.References.Equal(cache.References) {
		if len(cache.Snapshots) != 0 {
			logger.Verbosef("ERROR cosiHandleFinalization malformated head round references not empty %s %v %d\n", m.PeerId, s, len(cache.Snapshots))
			return nil
		}
		if s.References.Self != cache.References.Self {
			logger.Verbosef("ERROR cosiHandleFinalization malformated head round references self diff %s %v %v\n", m.PeerId, s, cache.References)
			return nil
		}
		external, err := chain.persistStore.ReadRound(s.References.External)
		if err != nil {
			return err
		}
		if external == nil {
			logger.Verbosef("ERROR cosiHandleFinalization head round references external not ready yet %s %v %v\n", m.PeerId, s, cache.References)
			return chain.QueueAppendSnapshot(m.PeerId, s, true)
		}
		err = chain.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, s.References)
		if err != nil {
			panic(err)
		}
		cache.References = s.References
		chain.assignNewGraphRound(final, cache)
		return chain.QueueAppendSnapshot(m.PeerId, s, true)
	}
	if s.RoundNumber == cache.Number+1 {
		if round, _, err := chain.startNewRound(s, cache, false); err != nil {
			return chain.QueueAppendSnapshot(m.PeerId, s, true)
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
	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: chain.node.TopoCounter.Next(),
	}
	for {
		err := chain.persistStore.WriteSnapshot(topo)
		if err != nil {
			logger.Debugf("ERROR cosiHandleFinalization WriteSnapshot %s %v %s\n", m.PeerId, m.Snapshot, err.Error())
			time.Sleep(10 * time.Millisecond)
		} else {
			break
		}
	}
	if err := cache.ValidateSnapshot(s, true); err != nil {
		panic("should never be here")
	}
	chain.assignNewGraphRound(final, cache)
	return chain.node.reloadConsensusNodesList(s, tx)
}

func (chain *Chain) handleFinalization(m *CosiAction) error {
	logger.Debugf("CosiLoop cosiHandleAction handleFinalization %s %v\n", m.PeerId, m.Snapshot)
	s := m.Snapshot
	s.Hash = s.PayloadHash()
	if !chain.node.verifyFinalization(s) {
		logger.Verbosef("ERROR handleFinalization verifyFinalization %s %v %d %t\n", m.PeerId, s, chain.node.ConsensusThreshold(s.Timestamp), chain.node.ConsensusRemovedRecently(s.Timestamp) != nil)
		return nil
	}

	if cache := chain.State.CacheRound; cache != nil {
		if s.RoundNumber < cache.Number {
			logger.Verbosef("ERROR handleFinalization expired round %s %s %d %d\n", m.PeerId, s.Hash, s.RoundNumber, cache.Number)
			return nil
		}
		if s.RoundNumber > cache.Number+1 {
			return chain.QueueAppendSnapshot(m.PeerId, s, true)
		}
	}

	dummy, err := chain.tryToStartNewRound(s)
	if err != nil {
		logger.Verbosef("ERROR handleFinalization tryToStartNewRound %s %s %d %t %s\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp), chain.node.ConsensusRemovedRecently(s.Timestamp) != nil, err.Error())
		return chain.QueueAppendSnapshot(m.PeerId, s, true)
	} else if dummy {
		logger.Verbosef("ERROR handleFinalization tryToStartNewRound DUMMY %s %s %d %t\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp), chain.node.ConsensusRemovedRecently(s.Timestamp) != nil)
		return chain.QueueAppendSnapshot(m.PeerId, s, true)
	}

	tx, err := chain.node.checkFinalSnapshotTransaction(s)
	if err != nil {
		logger.Verbosef("ERROR handleFinalization checkFinalSnapshotTransaction %s %s %d %t %s\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp), chain.node.ConsensusRemovedRecently(s.Timestamp) != nil, err.Error())
		return chain.QueueAppendSnapshot(m.PeerId, s, true)
	} else if tx == nil {
		logger.Verbosef("ERROR handleFinalization checkFinalSnapshotTransaction %s %s %d %t %s\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp), chain.node.ConsensusRemovedRecently(s.Timestamp) != nil, "tx empty")
		return nil
	}
	if s.RoundNumber == 0 && tx.TransactionType() != common.TransactionTypeNodeAccept {
		return fmt.Errorf("invalid initial transaction type %d", tx.TransactionType())
	}

	m.Transaction = tx
	return chain.cosiHandleFinalization(m)
}

func (node *Node) CosiQueueExternalAnnouncement(peerId crypto.Hash, s *common.Snapshot, commitment *crypto.Key) error {
	if node.getPeerConsensusNode(peerId) == nil {
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
	return chain.QueueAppendSnapshot(peerId, s, false)
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
	chain.cosiActionsChan <- m
	return nil
}

func (node *Node) CosiQueueExternalChallenge(peerId crypto.Hash, snap crypto.Hash, cosi *crypto.CosiSignature, ver *common.VersionedTransaction) error {
	if node.getPeerConsensusNode(peerId) == nil {
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
	chain.cosiActionsChan <- m
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
	chain.cosiActionsChan <- m
	return nil
}

func (node *Node) VerifyAndQueueAppendSnapshotFinalization(peerId crypto.Hash, s *common.Snapshot, timer *util.Timer) error {
	s.Hash = s.PayloadHash()
	logger.Debugf("VerifyAndQueueAppendSnapshotFinalization(%s, %s)\n", peerId, s.Hash)
	if node.custom.Node.ConsensusOnly && node.getPeerConsensusNode(peerId) == nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) invalid consensus peer\n", peerId, s.Hash)
		return nil
	}

	node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash)
	err := node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash, timer)
	if err != nil {
		return err
	}
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) already finalized %t %v\n", peerId, s.Hash, inNode, err)
		return err
	}

	hasTx, err := node.checkTxInStorage(s.Transaction)
	if err != nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) check tx error %v\n", peerId, s.Hash, err)
	} else if !hasTx {
		node.Peer.SendTransactionRequestMessage(peerId, s.Transaction, timer)
	}

	chain := node.GetOrCreateChain(s.NodeId)
	if s.Version == 0 {
		return chain.legacyAppendFinalization(peerId, s)
	}
	if !node.verifyFinalization(s) {
		logger.Verbosef("ERROR VerifyAndQueueAppendSnapshotFinalization %s %v %d %t\n", peerId, s, node.ConsensusThreshold(s.Timestamp), node.ConsensusRemovedRecently(s.Timestamp) != nil)
		return nil
	}

	return chain.QueueAppendSnapshot(peerId, s, true)
}

func (node *Node) getPeerConsensusNode(peerId crypto.Hash) *common.Node {
	if n := node.ConsensusPledging; n != nil && n.IdForNetwork(node.networkId) == peerId {
		return n
	}
	return node.ConsensusNodes[peerId]
}
