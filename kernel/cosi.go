package kernel

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
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
	defer node.Graph.UpdateFinalCache(node.IdForNetwork)

	switch m.Action {
	case CosiActionSelfEmpty:
		return node.cosiSendAnnouncement(m)
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

func (node *Node) cosiSendAnnouncement(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement %v\n", m.Snapshot)
	s := m.Snapshot
	if s.NodeId != node.IdForNetwork || s.NodeId != m.PeerId {
		panic("should never be here")
	}
	if s.Version != common.SnapshotVersion || s.Signature != nil || s.Timestamp != 0 {
		return nil
	}
	if !node.CheckCatchUpWithPeers() && !node.checkInitialAcceptSnapshotWeak(m.Snapshot) {
		return nil
	}

	tx, finalized, err := node.checkCacheSnapshotTransaction(s)
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

	if node.checkInitialAcceptSnapshot(s, tx) {
		s.Timestamp = uint64(clock.Now().UnixNano())
		s.Hash = s.PayloadHash()
		v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
		R := v.random.Public()
		node.CosiVerifiers[s.Hash] = v
		agg.Commitments[len(node.SortedConsensusNodes)] = &R
		node.CosiAggregators.Set(s.Hash, agg)
		node.CosiAggregators.Set(s.Transaction, agg)
		for peerId, _ := range node.ConsensusNodes {
			err := node.Peer.SendSnapshotAnnouncementMessage(peerId, s, R)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if node.ConsensusIndex < 0 || node.Graph.FinalRound[s.NodeId] == nil {
		return nil
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if len(cache.Snapshots) == 0 && !node.CheckBroadcastedToPeers() {
		return node.clearAndQueueSnapshotOrPanic(s)
	}
	for {
		s.Timestamp = uint64(clock.Now().UnixNano())
		if s.Timestamp > cache.Timestamp {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if len(cache.Snapshots) == 0 {
		external, err := node.persistStore.ReadRound(cache.References.External)
		if err != nil {
			return err
		}
		best := node.determinBestRound(s.Timestamp)
		threshold := external.Timestamp + config.SnapshotReferenceThreshold*config.SnapshotRoundGap*36
		if best != nil && best.NodeId != final.NodeId && threshold < best.Start {
			link, err := node.persistStore.ReadLink(cache.NodeId, best.NodeId)
			if err != nil {
				return err
			}
			if best.Number <= link {
				return node.clearAndQueueSnapshotOrPanic(s)
			}
			cache = &CacheRound{
				NodeId: cache.NodeId,
				Number: cache.Number,
				References: &common.RoundLink{
					Self:     final.Hash,
					External: best.Hash,
				},
			}
			err = node.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, cache.References)
			if err != nil {
				panic(err)
			}
			node.assignNewGraphRound(final, cache)
			return node.clearAndQueueSnapshotOrPanic(s)
		}
	} else if start, _ := cache.Gap(); s.Timestamp >= start+config.SnapshotRoundGap {
		best := node.determinBestRound(s.Timestamp)
		if best == nil {
			return node.clearAndQueueSnapshotOrPanic(s)
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
		err := node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
		node.CosiAggregators.Reset()
	}
	cache.Timestamp = s.Timestamp

	if node.CosiAggregators.Full(config.SnapshotRoundGap * 5 / 4) {
		return node.clearAndQueueSnapshotOrPanic(s)
	}
	if len(cache.Snapshots) > 0 && s.Timestamp > cache.Snapshots[0].Timestamp+uint64(config.SnapshotRoundGap*4/5) {
		return node.clearAndQueueSnapshotOrPanic(s)
	}

	s.RoundNumber = cache.Number
	s.References = cache.References
	s.Hash = s.PayloadHash()
	v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
	R := v.random.Public()
	node.CosiVerifiers[s.Hash] = v
	agg.Commitments[node.ConsensusIndex] = &R
	node.assignNewGraphRound(final, cache)
	node.CosiAggregators.Set(s.Hash, agg)
	node.CosiAggregators.Set(s.Transaction, agg)
	for peerId, _ := range node.ConsensusNodes {
		err := node.Peer.SendSnapshotAnnouncementMessage(peerId, m.Snapshot, R)
		if err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) cosiHandleAnnouncement(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v\n", m.PeerId, m.Snapshot)
	if node.ConsensusIndex < 0 || !node.CheckCatchUpWithPeers() {
		return nil
	}
	cn := node.getPeerConsensusNode(m.PeerId)
	if cn == nil {
		return nil
	}
	if cn.Timestamp+uint64(config.KernelNodeAcceptPeriodMinimum) >= m.Snapshot.Timestamp && !node.genesisNodesMap[cn.IdForNetwork(node.networkId)] {
		return nil
	}

	s := m.Snapshot
	if s.NodeId == node.IdForNetwork || s.NodeId != m.PeerId {
		panic(fmt.Errorf("should never be here %s %s %s", node.IdForNetwork, s.NodeId, s.Signature))
	}
	if s.Version != common.SnapshotVersion || s.Signature != nil || s.Timestamp == 0 {
		return nil
	}
	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if s.Timestamp > uint64(clock.Now().UnixNano())+threshold {
		return nil
	}
	if s.Timestamp+threshold*2 < node.Graph.GraphTimestamp {
		return nil
	}

	tx, finalized, err := node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized {
		return nil
	}

	v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
	if node.checkInitialAcceptSnapshotWeak(s) {
		node.CosiVerifiers[s.Hash] = v
		return node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.Hash, v.random.Public(), tx == nil)
	}

	if node.Graph.FinalRound[s.NodeId] == nil {
		return nil
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if s.RoundNumber < cache.Number {
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return nil
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
		external, err := node.persistStore.ReadRound(s.References.External)
		if err != nil || external == nil {
			return err
		}
		link, err := node.persistStore.ReadLink(cache.NodeId, external.NodeId)
		if err != nil {
			return err
		}
		if external.Number < link {
			return nil
		}
		cache = &CacheRound{
			NodeId: cache.NodeId,
			Number: cache.Number,
			References: &common.RoundLink{
				Self:     s.References.Self,
				External: s.References.External,
			},
		}
		err = node.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, cache.References)
		if err != nil {
			panic(err)
		}
		node.assignNewGraphRound(final, cache)
		return node.queueSnapshotOrPanic(m.PeerId, s)
	}
	if s.RoundNumber == cache.Number+1 {
		round, _, err := node.startNewRound(s, cache, false)
		if err != nil {
			logger.Verbosef("ERROR verifyExternalSnapshot %s %d %s %s\n", s.NodeId, s.RoundNumber, s.Transaction, err.Error())
			return node.queueSnapshotOrPanic(m.PeerId, s)
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
		err = node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
	}
	node.assignNewGraphRound(final, cache)

	if !cache.ValidateSnapshot(s, false) {
		return nil
	}

	node.CosiVerifiers[s.Hash] = v
	return node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.Hash, v.random.Public(), tx == nil)
}

func (node *Node) cosiHandleCommitment(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment %v\n", m)
	cn := node.ConsensusNodes[m.PeerId]
	if cn == nil {
		return nil
	}

	ann := node.CosiAggregators.Get(m.SnapshotHash)
	if ann == nil || ann.Snapshot.Hash != m.SnapshotHash {
		return nil
	}
	if ann.committed[m.PeerId] {
		return nil
	}
	if !node.CheckCatchUpWithPeers() && !node.checkInitialAcceptSnapshotWeak(ann.Snapshot) {
		return nil
	}
	if cn.Timestamp+uint64(config.KernelNodeAcceptPeriodMinimum) >= ann.Snapshot.Timestamp && !node.genesisNodesMap[cn.IdForNetwork(node.networkId)] {
		return nil
	}
	ann.committed[m.PeerId] = true

	base := node.ConsensusThreshold(ann.Snapshot.Timestamp)
	if len(ann.Commitments) >= base {
		return nil
	}
	for i, id := range node.SortedConsensusNodes {
		if id == m.PeerId {
			ann.Commitments[i] = m.Commitment
			ann.WantTxs[m.PeerId] = m.WantTx
			break
		}
	}
	if len(ann.Commitments) < base {
		return nil
	}

	tx, finalized, err := node.checkCacheSnapshotTransaction(ann.Snapshot)
	if err != nil || finalized || tx == nil {
		return nil
	}

	cosi, err := crypto.CosiAggregateCommitment(ann.Commitments)
	if err != nil {
		return err
	}
	ann.Snapshot.Signature = cosi
	v := node.CosiVerifiers[m.SnapshotHash]
	priv := node.Signer.PrivateSpendKey
	publics := node.ConsensusKeys(ann.Snapshot.Timestamp)
	if node.checkInitialAcceptSnapshot(ann.Snapshot, tx) {
		publics = append(publics, &node.ConsensusPledging.Signer.PublicSpendKey)
	}
	response, err := cosi.Response(&priv, v.random, publics, m.SnapshotHash[:])
	if err != nil {
		return err
	}
	if node.checkInitialAcceptSnapshot(ann.Snapshot, tx) {
		ann.Responses[len(node.SortedConsensusNodes)] = &response
	} else {
		ann.Responses[node.ConsensusIndex] = &response
	}
	copy(cosi.Signature[32:], response[:])
	for id, _ := range node.ConsensusNodes {
		if wantTx, found := ann.WantTxs[id]; !found {
			continue
		} else if wantTx {
			err = node.Peer.SendTransactionChallengeMessage(id, m.SnapshotHash, cosi, tx)
		} else {
			err = node.Peer.SendTransactionChallengeMessage(id, m.SnapshotHash, cosi, nil)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) cosiHandleChallenge(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleChallenge %v\n", m)
	if node.ConsensusIndex < 0 || !node.CheckCatchUpWithPeers() {
		return nil
	}
	if node.getPeerConsensusNode(m.PeerId) == nil {
		return nil
	}

	v := node.CosiVerifiers[m.SnapshotHash]
	if v == nil || v.Snapshot.Hash != m.SnapshotHash {
		return nil
	}

	if m.Transaction != nil {
		err := node.CachePutTransaction(m.PeerId, m.Transaction)
		if err != nil {
			return err
		}
	}

	s := v.Snapshot
	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if s.Timestamp > uint64(clock.Now().UnixNano())+threshold {
		return nil
	}
	if s.Timestamp+threshold*2 < node.Graph.GraphTimestamp {
		return nil
	}

	tx, finalized, err := node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized || tx == nil {
		return nil
	}

	var sig crypto.Signature
	copy(sig[:], s.Commitment[:])
	copy(sig[32:], m.Signature.Signature[32:])
	pub := node.getPeerConsensusNode(s.NodeId).Signer.PublicSpendKey
	publics := node.ConsensusKeys(s.Timestamp)
	if node.checkInitialAcceptSnapshot(s, tx) {
		publics = append(publics, &node.ConsensusPledging.Signer.PublicSpendKey)
	}
	challenge, err := m.Signature.Challenge(publics, m.SnapshotHash[:])
	if err != nil {
		return nil
	}
	if !pub.VerifyWithChallenge(m.SnapshotHash[:], sig, challenge) {
		return nil
	}

	priv := node.Signer.PrivateSpendKey
	response, err := m.Signature.Response(&priv, v.random, publics, m.SnapshotHash[:])
	if err != nil {
		return err
	}
	return node.Peer.SendSnapshotResponseMessage(m.PeerId, m.SnapshotHash, response)
}

func (node *Node) cosiHandleResponse(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v\n", m)
	if node.ConsensusNodes[m.PeerId] == nil {
		return nil
	}

	agg := node.CosiAggregators.Get(m.SnapshotHash)
	if agg == nil || agg.Snapshot.Hash != m.SnapshotHash {
		return nil
	}
	if agg.responsed[m.PeerId] {
		return nil
	}
	if !node.CheckCatchUpWithPeers() && !node.checkInitialAcceptSnapshotWeak(agg.Snapshot) {
		return nil
	}
	agg.responsed[m.PeerId] = true
	if len(agg.Responses) >= len(agg.Commitments) {
		return nil
	}
	base := node.ConsensusThreshold(agg.Snapshot.Timestamp)
	if len(agg.Commitments) < base {
		return nil
	}

	s := agg.Snapshot
	tx, finalized, err := node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized || tx == nil {
		return nil
	}

	for i, id := range node.SortedConsensusNodes {
		if id == m.PeerId {
			agg.Responses[i] = m.Response
			break
		}
	}
	if len(agg.Responses) != len(agg.Commitments) {
		return nil
	}
	node.CosiAggregators.Delete(agg.Snapshot.Hash)
	node.CosiAggregators.Delete(agg.Snapshot.Transaction)

	publics := node.ConsensusKeys(s.Timestamp)
	if node.checkInitialAcceptSnapshot(s, tx) {
		publics = append(publics, &node.ConsensusPledging.Signer.PublicSpendKey)
	}
	s.Signature.AggregateResponse(publics, agg.Responses, m.SnapshotHash[:], false)
	if !node.CacheVerifyCosi(m.SnapshotHash, s.Signature, publics, base) {
		return nil
	}

	if node.checkInitialAcceptSnapshot(s, tx) {
		err := node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
		for id, _ := range node.ConsensusNodes {
			err := node.Peer.SendSnapshotFinalizationMessage(id, s)
			if err != nil {
				return err
			}
		}
		return node.reloadConsensusNodesList(s, tx)
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	if s.RoundNumber > cache.Number {
		panic(fmt.Sprintf("should never be here %d %d", cache.Number, s.RoundNumber))
	}
	if s.RoundNumber < cache.Number {
		return node.clearAndQueueSnapshotOrPanic(s)
	}
	if !s.References.Equal(cache.References) {
		return node.clearAndQueueSnapshotOrPanic(s)
	}
	if !cache.ValidateSnapshot(s, false) {
		return node.clearAndQueueSnapshotOrPanic(s)
	}

	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: node.TopoCounter.Next(),
	}
	err = node.persistStore.WriteSnapshot(topo)
	if err != nil {
		panic(err)
	}
	if !cache.ValidateSnapshot(s, true) {
		panic("should never be here")
	}
	node.Graph.CacheRound[s.NodeId] = cache

	for id, _ := range node.ConsensusNodes {
		err := node.Peer.SendSnapshotFinalizationMessage(id, agg.Snapshot)
		if err != nil {
			return err
		}
	}
	return node.reloadConsensusNodesList(s, tx)
}

func (node *Node) cosiHandleFinalization(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleFinalization %s %v\n", m.PeerId, m.Snapshot)
	s, tx := m.Snapshot, m.Transaction

	if node.checkInitialAcceptSnapshot(s, tx) {
		err := node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
		return node.reloadConsensusNodesList(s, tx)
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if s.RoundNumber < cache.Number {
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return node.QueueAppendSnapshot(m.PeerId, s, true)
	}
	if s.RoundNumber == cache.Number && !s.References.Equal(cache.References) {
		if s.NodeId == node.IdForNetwork {
			return nil
		}
		if len(cache.Snapshots) != 0 {
			return nil
		}
		external, err := node.persistStore.ReadRound(s.References.External)
		if err != nil || external == nil {
			return err
		}
		err = node.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, s.References)
		if err != nil {
			panic(err)
		}
		cache.References = s.References
		node.assignNewGraphRound(final, cache)
		return node.QueueAppendSnapshot(m.PeerId, s, true)
	}
	if s.RoundNumber == cache.Number+1 {
		if round, _, err := node.startNewRound(s, cache, false); err != nil {
			return node.QueueAppendSnapshot(m.PeerId, s, true)
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
		err := node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
	}
	node.assignNewGraphRound(final, cache)

	if !cache.ValidateSnapshot(s, false) {
		return nil
	}
	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: node.TopoCounter.Next(),
	}
	err := node.persistStore.WriteSnapshot(topo)
	if err != nil {
		panic(err)
	}
	if !cache.ValidateSnapshot(s, true) {
		panic("should never be here")
	}
	node.assignNewGraphRound(final, cache)
	return node.reloadConsensusNodesList(s, tx)
}

func (node *Node) handleFinalization(m *CosiAction) error {
	logger.Verbosef("CosiLoop cosiHandleAction handleFinalization %s %v\n", m.PeerId, m.Snapshot)
	s := m.Snapshot
	s.Hash = s.PayloadHash()
	if !node.verifyFinalization(s) {
		logger.Verbosef("ERROR handleFinalization verifyFinalization %s %d %t\n", s.Hash, node.ConsensusThreshold(s.Timestamp), node.ConsensusRemoved != nil)
		return nil
	}

	dummy, err := node.tryToStartNewRound(s)
	if err != nil {
		logger.Verbosef("ERROR handleFinalization tryToStartNewRound %s %d %t %s\n", s.Hash, node.ConsensusThreshold(s.Timestamp), node.ConsensusRemoved != nil, err.Error())
		return node.QueueAppendSnapshot(m.PeerId, s, true)
	} else if dummy {
		logger.Verbosef("ERROR handleFinalization tryToStartNewRound DUMMY %s %d %t\n", s.Hash, node.ConsensusThreshold(s.Timestamp), node.ConsensusRemoved != nil)
		return node.QueueAppendSnapshot(m.PeerId, s, true)
	}

	tx, err := node.checkFinalSnapshotTransaction(s)
	if err != nil {
		logger.Verbosef("ERROR handleFinalization checkFinalSnapshotTransaction %s %d %t %s\n", s.Hash, node.ConsensusThreshold(s.Timestamp), node.ConsensusRemoved != nil, err.Error())
		return node.QueueAppendSnapshot(m.PeerId, s, true)
	} else if tx == nil {
		logger.Verbosef("ERROR handleFinalization checkFinalSnapshotTransaction %s %d %t %s\n", s.Hash, node.ConsensusThreshold(s.Timestamp), node.ConsensusRemoved != nil, "tx empty")
		return nil
	}
	if s.RoundNumber == 0 && tx.TransactionType() != common.TransactionTypeNodeAccept {
		return fmt.Errorf("invalid initial transaction type %d", tx.TransactionType())
	}

	m.Transaction = tx
	return node.cosiHandleFinalization(m)
}

func (node *Node) checkAnnouncementFlood(s *common.Snapshot, duration time.Duration) bool {
	now := clock.Now()
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, s.RoundNumber)
	txForNode := s.Transaction.ForNetwork(s.NodeId)
	key := append(txForNode[:], buf...)

	val := node.cacheStore.Get(nil, key)
	if len(val) != 8 {
		return false
	}
	ts := time.Unix(0, int64(binary.BigEndian.Uint64(val)))
	if ts.Add(duration).After(now) {
		return true
	}

	binary.BigEndian.PutUint64(buf, uint64(now.UnixNano()))
	node.cacheStore.Set(key, buf)
	return false
}

func (node *Node) CosiQueueExternalAnnouncement(peerId crypto.Hash, s *common.Snapshot, commitment *crypto.Key) error {
	if node.getPeerConsensusNode(peerId) == nil {
		return nil
	}
	if node.checkAnnouncementFlood(s, time.Duration(config.SnapshotRoundGap)) {
		return nil
	}

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
	return node.QueueAppendSnapshot(peerId, s, false)
}

func (node *Node) CosiAggregateSelfCommitments(peerId crypto.Hash, snap crypto.Hash, commitment *crypto.Key, wantTx bool) error {
	if node.ConsensusNodes[peerId] == nil {
		return nil
	}

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionSelfCommitment,
		SnapshotHash: snap,
		Commitment:   commitment,
		WantTx:       wantTx,
	}
	node.cosiActionsChan <- m
	return nil
}

func (node *Node) CosiQueueExternalChallenge(peerId crypto.Hash, snap crypto.Hash, cosi *crypto.CosiSignature, ver *common.VersionedTransaction) error {
	if node.getPeerConsensusNode(peerId) == nil {
		return nil
	}

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionExternalChallenge,
		SnapshotHash: snap,
		Signature:    cosi,
		Transaction:  ver,
	}
	node.cosiActionsChan <- m
	return nil
}

func (node *Node) CosiAggregateSelfResponses(peerId crypto.Hash, snap crypto.Hash, response *[32]byte) error {
	if node.ConsensusNodes[peerId] == nil {
		return nil
	}

	agg := node.CosiAggregators.Get(snap)
	if agg == nil {
		return nil
	}

	s := agg.Snapshot
	tx, finalized, err := node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized || tx == nil {
		return nil
	}

	index := -1
	for i, id := range node.SortedConsensusNodes {
		if id == peerId {
			index = i
			break
		}
	}
	if index < 0 {
		return nil
	}
	publics := node.ConsensusKeys(s.Timestamp)
	if node.checkInitialAcceptSnapshotWeak(s) {
		publics = append(publics, &node.ConsensusPledging.Signer.PublicSpendKey)
	}
	err = s.Signature.VerifyResponse(publics, index, response, snap[:])
	if err != nil {
		return nil
	}

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionSelfResponse,
		SnapshotHash: snap,
		Response:     response,
	}
	node.cosiActionsChan <- m
	return nil
}

func (node *Node) VerifyAndQueueAppendSnapshotFinalization(peerId crypto.Hash, s *common.Snapshot) error {
	s.Hash = s.PayloadHash()
	logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s)\n", peerId, s.Hash)
	if node.getPeerConsensusNode(peerId) == nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) invalid consensus peer\n", peerId, s.Hash)
		return nil
	}
	if swt, err := node.persistStore.CheckSnapshot(s.Hash); err != nil {
		return err
	} else if swt {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) already snapshot\n", peerId, s.Hash)
		node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash)
		return node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash)
	}

	if s.Version == 0 {
		return node.legacyAppendFinalization(peerId, s)
	}
	if !node.verifyFinalization(s) {
		logger.Verbosef("ERROR VerifyAndQueueAppendSnapshotFinalization %s %d %t\n", s.Hash, node.ConsensusThreshold(s.Timestamp), node.ConsensusRemoved != nil)
		return nil
	}

	node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash)
	err := node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash)
	if err != nil {
		return err
	}

	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) already finalized %t %v\n", peerId, s.Hash, inNode, err)
		return err
	}
	return node.QueueAppendSnapshot(peerId, s, true)
}

func (node *Node) getPeerConsensusNode(peerId crypto.Hash) *common.Node {
	if n := node.ConsensusPledging; n != nil && n.IdForNetwork(node.networkId) == peerId {
		return n
	}
	return node.ConsensusNodes[peerId]
}

type aggregatorMap struct {
	mutex *sync.RWMutex
	m     map[crypto.Hash]*CosiAggregator
}

func (s *aggregatorMap) Set(k crypto.Hash, p *CosiAggregator) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.m[k] = p
}

func (s *aggregatorMap) Get(k crypto.Hash) *CosiAggregator {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.m[k]
}

func (s *aggregatorMap) Delete(k crypto.Hash) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.m, k)
}

func (s *aggregatorMap) Full(threshold uint64) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	expired, total := 0, 0
	now := uint64(clock.Now().UnixNano())
	for _, agg := range s.m {
		if agg.Snapshot.Timestamp+threshold < now {
			expired++
		}
		total++
	}
	return total >= config.SnapshotRoundSize && expired < total*4/5
}

func (s *aggregatorMap) Reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.m = make(map[crypto.Hash]*CosiAggregator, config.SnapshotRoundSize)
}
