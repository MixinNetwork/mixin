package kernel

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
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
	if node.ConsensusIndex < 0 || !node.CheckCatchUpWithPeers() {
		time.Sleep(100 * time.Millisecond)
		return node.queueSnapshotOrPanic(m.PeerId, m.Snapshot, false)
	}

	s := m.Snapshot
	if s.NodeId != node.IdForNetwork || s.NodeId != m.PeerId {
		panic("should never be here")
	}
	if s.Version != common.SnapshotVersion || s.Signature != nil || s.Timestamp != 0 {
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
		s.Timestamp = uint64(time.Now().UnixNano())
		s.Hash = s.PayloadHash()
		v := &CosiVerifier{Snapshot: s, random: crypto.CosiCommit(rand.Reader)}
		R := v.random.Public()
		node.CosiVerifiers[s.Hash] = v
		agg.Commitments[node.ConsensusIndex] = &R
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

	if node.CosiAggregators.Get(s.Transaction) != nil {
		return nil
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if len(cache.Snapshots) == 0 && !node.CheckBroadcastedToPeers() {
		time.Sleep(time.Duration(config.SnapshotRoundGap / 2))
		return node.queueSnapshotOrPanic(m.PeerId, s, false)
	}
	for {
		s.Timestamp = uint64(time.Now().UnixNano())
		if s.Timestamp > cache.Timestamp {
			break
		}
		time.Sleep(300 * time.Millisecond)
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
			time.Sleep(time.Duration(config.SnapshotRoundGap / 2))
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
	}
	cache.Timestamp = s.Timestamp

	// TODO check cache snapshot timestamp gap*2/3

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
	if node.ConsensusIndex < 0 || !node.CheckCatchUpWithPeers() {
		time.Sleep(100 * time.Millisecond)
		return nil
	}
	cn := node.ConsensusNodes[m.PeerId]
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
	if s.Timestamp > uint64(time.Now().UnixNano())+threshold {
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
	if node.checkInitialAcceptSnapshot(s, tx) {
		node.CosiVerifiers[s.Hash] = v
		return node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.Hash, v.random.Public(), tx == nil)
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if s.RoundNumber < cache.Number {
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return node.queueSnapshotOrPanic(m.PeerId, s, false)
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
		old, err := node.persistStore.ReadRound(cache.References.External)
		if err != nil {
			return err
		}
		external, err := node.persistStore.ReadRound(s.References.External)
		if err != nil || external == nil {
			return err
		}
		if old.Timestamp+config.SnapshotReferenceThreshold*config.SnapshotRoundGap*32 > external.Timestamp {
			return nil
		}
		link, err := node.persistStore.ReadLink(cache.NodeId, external.NodeId)
		if err != nil {
			return err
		}
		if external.Number <= link {
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
		return node.queueSnapshotOrPanic(m.PeerId, s, false)
	}
	if s.RoundNumber == cache.Number+1 {
		if round, err := node.startNewRound(s, cache); err != nil {
			logger.Verbosef("ERROR verifyExternalSnapshot %s %d %s %s %s\n", s.NodeId, s.RoundNumber, s.References.Self, s.References.External, err.Error())
			return node.queueSnapshotOrPanic(m.PeerId, s, false)
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

	node.CosiVerifiers[s.Hash] = v
	return node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s.Hash, v.random.Public(), tx == nil)
}

func (node *Node) cosiHandleCommitment(m *CosiAction) error {
	if node.ConsensusIndex < 0 || !node.CheckCatchUpWithPeers() {
		time.Sleep(100 * time.Millisecond)
		return nil
	}
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
	response, err := cosi.Response(&priv, v.random, publics, m.SnapshotHash[:])
	if err != nil {
		return err
	}
	ann.Responses[node.ConsensusIndex] = &response
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
	if node.ConsensusIndex < 0 || !node.CheckCatchUpWithPeers() {
		time.Sleep(100 * time.Millisecond)
		return nil
	}
	if node.ConsensusNodes[m.PeerId] == nil {
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
	tx, finalized, err := node.checkCacheSnapshotTransaction(s)
	if err != nil || finalized || tx == nil {
		return nil
	}

	var sig crypto.Signature
	copy(sig[:], s.Commitment[:])
	copy(sig[32:], m.Signature.Signature[32:])
	pub := node.ConsensusNodes[s.NodeId].Signer.PublicSpendKey
	publics := node.ConsensusKeys(s.Timestamp)
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
	if node.ConsensusIndex < 0 || !node.CheckCatchUpWithPeers() {
		time.Sleep(100 * time.Millisecond)
		return nil
	}
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

	publics := node.ConsensusKeys(s.Timestamp)
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
		return node.queueSnapshotOrPanic(m.PeerId, s, true)
	}
	if s.RoundNumber == cache.Number && !s.References.Equal(cache.References) {
		if s.NodeId == node.IdForNetwork {
			return nil
		}
		if len(cache.Snapshots) != 0 {
			return nil
		}
		err := node.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, s.References)
		if err != nil {
			panic(err)
		}
		cache.References = s.References
		node.assignNewGraphRound(final, cache)
		return node.queueSnapshotOrPanic(m.PeerId, s, true)
	}
	if s.RoundNumber == cache.Number+1 {
		if round, err := node.startNewRound(s, cache); err != nil {
			return node.queueSnapshotOrPanic(m.PeerId, s, true)
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
	s := m.Snapshot
	s.Hash = s.PayloadHash()
	if !node.verifyFinalization(s) {
		return nil
	}

	err := node.tryToStartNewRound(s)
	if err != nil {
		return node.queueSnapshotOrPanic(m.PeerId, s, true)
	}

	tx, err := node.checkFinalSnapshotTransaction(s)
	if err != nil {
		return node.queueSnapshotOrPanic(m.PeerId, s, true)
	} else if tx == nil {
		return nil
	}
	m.Transaction = tx
	return node.cosiHandleFinalization(m)
}

func (node *Node) CosiQueueExternalAnnouncement(peerId crypto.Hash, s *common.Snapshot, commitment *crypto.Key) error {
	if node.ConsensusNodes[s.NodeId] == nil {
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
	if node.ConsensusNodes[peerId] == nil {
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
	if node.ConsensusNodes[peerId] == nil {
		return nil
	}

	if s.Version == 0 {
		return node.legacyAppendFinalization(peerId, s)
	}
	if s.Version != common.SnapshotVersion || s.Signature == nil {
		return nil
	}

	s.Hash = s.PayloadHash()
	publics := node.ConsensusKeys(s.Timestamp)
	base := node.ConsensusThreshold(s.Timestamp)
	if !node.CacheVerifyCosi(s.Hash, s.Signature, publics, base) {
		return nil
	}

	node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash)
	err := node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash)
	if err != nil {
		return err
	}

	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return err
	}
	return node.QueueAppendSnapshot(peerId, s, true)
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
