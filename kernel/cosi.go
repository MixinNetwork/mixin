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
	logger.Debugf("cosiHook(%s) %v\n", chain.ChainId, m)
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
		logger.Debugf("cosiHandleAction checkActionSanity %v ERROR %s\n", m, err)
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
		ov := chain.CosiVerifiers[s.Transaction]
		if ov != nil && s.RoundNumber > 0 && ov.Snapshot.RoundNumber == s.RoundNumber && s.Timestamp < ov.Snapshot.Timestamp+config.SnapshotRoundGap {
			return fmt.Errorf("a transaction %s only in one round %d of one chain %s", s.Transaction, s.RoundNumber, chain.ChainId)
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

	if chain.IsPledging() && s.RoundNumber == 0 {
	} else if m.Action == CosiActionSelfEmpty {
		if !chain.node.CheckBroadcastedToPeers() {
			return fmt.Errorf("chain not broadcasted to peers yet")
		}
	} else {
		if chain.State == nil {
			return fmt.Errorf("state empty")
		}
		cache, final := chain.StateCopy()
		if s.RoundNumber < cache.Number {
			return fmt.Errorf("round stale %d %d", s.RoundNumber, cache.Number)
		}
		if s.RoundNumber > cache.Number+1 {
			return fmt.Errorf("round future %d %d", s.RoundNumber, cache.Number)
		}
		if s.Timestamp <= final.Start+config.SnapshotRoundGap {
			return fmt.Errorf("round timestamp invalid %d %d", s.Timestamp, final.Start+config.SnapshotRoundGap)
		}
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
	}

	if !chain.IsPledging() && !chain.node.CheckCatchUpWithPeers() {
		return fmt.Errorf("node is slow in catching up")
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
	} else if chain.State == nil {
		return nil
	} else {
		cache, final := chain.StateCopy()
		if len(cache.Snapshots) == 0 && !chain.node.CheckBroadcastedToPeers() {
			return nil
		}
		if s.Timestamp <= cache.Timestamp {
			return chain.clearAndQueueSnapshotOrPanic(s)
		}

		if len(cache.Snapshots) == 0 {
			external, err := chain.persistStore.ReadRound(cache.References.External)
			if err != nil {
				return err
			}
			best := chain.determinBestRound(s.Timestamp)
			threshold := external.Timestamp + config.SnapshotReferenceThreshold*config.SnapshotRoundGap*36
			if best != nil && best.NodeId != final.NodeId && threshold < best.Start {
				logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement new best external %s:%d:%d => %s:%d:%d\n", external.NodeId, external.Number, external.Timestamp, best.NodeId, best.Number, best.Start)
				references := &common.RoundLink{Self: final.Hash, External: best.Hash}
				err := chain.updateEmptyHeadRoundAndPersist(m, final, cache, references, s.Timestamp, true)
				if err != nil {
					logger.Verbosef("ERROR cosiHandleFinalization updateEmptyHeadRoundAndPersist failed %s %s %v\n", m.PeerId, s.Hash, err)
					return nil
				}
				return chain.clearAndQueueSnapshotOrPanic(s)
			}
		} else if start, _ := cache.Gap(); s.Timestamp >= start+config.SnapshotRoundGap {
			best := chain.determinBestRound(s.Timestamp)
			if best == nil {
				logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement no best available\n")
				return chain.clearAndQueueSnapshotOrPanic(s)
			}
			if best.NodeId == final.NodeId {
				panic("should never be here")
			}
			references := &common.RoundLink{Self: cache.asFinal().Hash, External: best.Hash}
			nc, nf, _, err := chain.startNewRoundAndPersist(cache, references, s.Timestamp, false)
			if err != nil || nf == nil {
				logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement %s %v startNewRoundAndPersist %v %v\n", m.PeerId, m.Snapshot, err, nf)
				return chain.clearAndQueueSnapshotOrPanic(s)
			}
			cache, final = nc, nf
		}
		cache.Timestamp = s.Timestamp

		if len(cache.Snapshots) > 0 {
			cft := cache.Snapshots[0].Timestamp
			if s.Timestamp > cft+uint64(config.SnapshotRoundGap*4/5) {
				return chain.clearAndQueueSnapshotOrPanic(s)
			}
			day := uint64(time.Hour) * 24
			if s.Timestamp/day != cft/day {
				return chain.clearAndQueueSnapshotOrPanic(s)
			}
		}
		s.RoundNumber = cache.Number
		s.References = cache.References
	}

	ov := chain.CosiVerifiers[s.Transaction]
	if ov != nil && s.RoundNumber > 0 && ov.Snapshot.RoundNumber == s.RoundNumber && s.Timestamp < ov.Snapshot.Timestamp+config.SnapshotRoundGap {
		err := fmt.Errorf("a transaction %s only in one round %d of one chain %s", s.Transaction, s.RoundNumber, chain.ChainId)
		logger.Verbosef("CosiLoop cosiHandleAction cosiSendAnnouncement ERROR %s\n", err)
		return nil
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
	chain.CosiVerifiers[s.Transaction] = v
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
	} else if chain.State == nil {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v empty final round\n", m.PeerId, m.Snapshot)
		return nil
	} else {
		cache, final := chain.StateCopy()
		if s.RoundNumber < cache.Number {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v expired %d %d\n", m.PeerId, m.Snapshot, s.RoundNumber, cache.Number)
			return nil
		}
		if s.RoundNumber > cache.Number+1 {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v in future %d %d\n", m.PeerId, m.Snapshot, s.RoundNumber, cache.Number)
			return nil
		}
		if s.Timestamp <= final.Start+config.SnapshotRoundGap {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v invalid timestamp %d %d\n", m.PeerId, m.Snapshot, s.Timestamp, final.Start+config.SnapshotRoundGap)
			return nil
		}
		if s.RoundNumber == cache.Number && !s.References.Equal(cache.References) {
			err := chain.updateEmptyHeadRoundAndPersist(m, final, cache, s.References, s.Timestamp, true)
			if err != nil {
				logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v updateEmptyHeadRoundAndPersist %v\n", m.PeerId, m.Snapshot, err)
				return nil
			}
			return chain.AppendCosiAction(m)
		}
		if s.RoundNumber == cache.Number+1 {
			nc, nf, _, err := chain.startNewRoundAndPersist(cache, s.References, s.Timestamp, false)
			if err != nil {
				logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v startNewRoundAndPersist %s\n", m.PeerId, m.Snapshot, err)
				return chain.AppendCosiAction(m)
			} else if nf == nil {
				logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v startNewRoundAndPersist failed\n", m.PeerId, m.Snapshot)
				return nil
			}
			cache, final = nc, nf
		}

		if err := cache.ValidateSnapshot(s); err != nil {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleAnnouncement %s %v ValidateSnapshot %s\n", m.PeerId, m.Snapshot, err)
			return nil
		}
	}

	r := crypto.CosiCommit(rand.Reader)
	v := &CosiVerifier{Snapshot: s, Commitment: m.Commitment, random: r}
	chain.CosiVerifiers[s.Hash] = v
	chain.CosiVerifiers[s.Transaction] = v
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
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment %v REPEAT\n", m)
		return nil
	}
	base := chain.node.ConsensusThreshold(ann.Snapshot.Timestamp)
	if len(ann.Commitments) >= base {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment %v EXCEED\n", m)
		return nil
	}
	ann.Commitments[cd.PN.ConsensusIndex] = m.Commitment
	ann.WantTxs[m.PeerId] = m.WantTx
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment %v NOW %d %d\nn", m, len(ann.Commitments), base)
	if len(ann.Commitments) < base {
		return nil
	}
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleCommitment %v ENOUGH\n", m)

	cosi, err := crypto.CosiAggregateCommitment(ann.Commitments)
	if err != nil {
		return err
	}
	s.Signature = cosi
	v := chain.CosiVerifiers[m.SnapshotHash]
	priv := chain.node.Signer.PrivateSpendKey
	_, publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	response, err := cosi.Response(&priv, v.random, publics, m.SnapshotHash[:])
	if err != nil {
		return err
	}
	ann.Responses[cd.CN.ConsensusIndex] = response
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
	_, publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	challenge, err := m.Signature.Challenge(publics, m.SnapshotHash[:])
	if err != nil {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleChallenge %v Challenge ERROR %s\n", m, err)
		return nil
	}
	if !pub.VerifyWithChallenge(m.SnapshotHash[:], sig, challenge) {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleChallenge %v VerifyWithChallenge ERROR\n", m)
		return nil
	}

	priv := chain.node.Signer.PrivateSpendKey
	response, err := m.Signature.Response(&priv, v.random, publics, m.SnapshotHash[:])
	if err != nil {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleChallenge %v Response ERROR %s\n", m, err)
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
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v REPEAT\n", m)
		return nil
	}
	if len(agg.Responses) >= len(agg.Commitments) {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v EXCEED\n", m)
		return nil
	}
	base := chain.node.ConsensusThreshold(s.Timestamp)
	agg.Responses[cd.PN.ConsensusIndex] = m.Response
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v NOW %d %d %d\n", m, len(agg.Responses), len(agg.Commitments), base)
	if len(agg.Responses) != len(agg.Commitments) {
		return nil
	}
	logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v ENOUGH\n", m)

	cids, publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	err := s.Signature.VerifyResponse(publics, cd.PN.ConsensusIndex, m.Response, m.SnapshotHash[:])
	if err != nil {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v RESPONSE ERROR %s\n", m, err)
		return nil
	}

	s.Signature.AggregateResponse(publics, agg.Responses, m.SnapshotHash[:], false)
	signers, finalized := chain.node.CacheVerifyCosi(m.SnapshotHash, s.Signature, cids, publics, base)
	if !finalized {
		logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v AGGREGATE ERROR\n", m)
		return nil
	}

	if chain.IsPledging() && s.RoundNumber == 0 && cd.TX.TransactionType() == common.TransactionTypeNodeAccept {
		err := chain.node.finalizeNodeAcceptSnapshot(s, signers)
		if err != nil {
			return err
		}
	} else {
		cache, final := chain.StateCopy()
		if s.RoundNumber > cache.Number {
			panic(fmt.Sprintf("should never be here %d %d", cache.Number, s.RoundNumber))
		}
		if s.RoundNumber < cache.Number {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v EXPIRE %d %d\n", m, s.RoundNumber, cache.Number)
			return nil
		}
		if !s.References.Equal(cache.References) {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v REFERENCES %v %v\n", m, s.References, cache.References)
			return nil
		}
		if err := cache.ValidateSnapshot(s); err != nil {
			logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse %v ValidateSnapshot %s\n", m, err)
			return nil
		}

		chain.AddSnapshot(final, cache, s, signers)
	}

	nodes := chain.node.NodesListWithoutState(s.Timestamp, true)
	for _, cn := range nodes {
		id := cn.IdForNetwork
		if agg.Responses[cn.ConsensusIndex] == nil {
			err := chain.node.SendTransactionToPeer(id, s.Transaction)
			if err != nil {
				logger.Verbosef("CosiLoop cosiHandleAction cosiHandleResponse SendTransactionToPeer(%s, %s) ERROR %s\n", id, m.SnapshotHash, err.Error())
			}
		}
		err := chain.node.Peer.SendSnapshotFinalizationMessage(id, s)
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

	if chain.IsPledging() && s.RoundNumber == 0 {
	} else if chain.State == nil {
		logger.Debugf("ERROR cosiHandleFinalization without consensus%s %s\n", m.PeerId, s.Hash)
		return nil
	} else {
		cache := chain.State.CacheRound
		if s.RoundNumber < cache.Number {
			logger.Debugf("ERROR cosiHandleFinalization expired round %s %s %d %d\n", m.PeerId, s.Hash, s.RoundNumber, cache.Number)
			return nil
		}
		if s.RoundNumber > cache.Number+1 {
			logger.Debugf("ERROR cosiHandleFinalization in future %s %s %d %d\n", m.PeerId, s.Hash, s.RoundNumber, cache.Number)
			return nil
		}
		if s.RoundNumber == cache.Number+1 {
			_, nf, dummy, err := chain.startNewRoundAndPersist(cache, s.References, s.Timestamp, true)
			if err != nil || nf == nil {
				logger.Verbosef("ERROR cosiHandleFinalization startNewRound %s %v %v %v\n", m.PeerId, s, err, nf)
				return nil
			}
			if dummy {
				logger.Verbosef("ERROR handleFinalization startNewRound DUMMY %s %s %d\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp))
				return nil
			}
		}
	}

	signers, finalized := chain.verifyFinalization(s)
	if !finalized {
		logger.Verbosef("ERROR handleFinalization verifyFinalization %s %v %d\n", m.PeerId, s, chain.node.ConsensusThreshold(s.Timestamp))
		return nil
	}

	tx, _, err := chain.node.validateSnapshotTransaction(s, true)
	if err != nil {
		logger.Verbosef("ERROR handleFinalization checkFinalSnapshotTransaction %s %s %d %s\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp), err.Error())
		return nil
	} else if tx == nil {
		logger.Verbosef("ERROR handleFinalization checkFinalSnapshotTransaction %s %s %d %s\n", m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp), "tx empty")
		m.WantTx = true
		return nil
	}

	if chain.IsPledging() && s.RoundNumber == 0 && tx.TransactionType() == common.TransactionTypeNodeAccept {
		err := chain.node.finalizeNodeAcceptSnapshot(s, signers)
		if err != nil {
			return err
		}
		return chain.node.reloadConsensusNodesList(s, tx)
	} else if chain.State == nil {
		return nil
	}

	cache, final := chain.StateCopy()
	if !s.References.Equal(cache.References) {
		err := chain.updateEmptyHeadRoundAndPersist(m, final, cache, s.References, s.Timestamp, false)
		if err != nil {
			logger.Debugf("ERROR cosiHandleFinalization updateEmptyHeadRoundAndPersist failed %s %s %v\n", m.PeerId, s.Hash, err)
		}
		return nil
	}

	if err := cache.ValidateSnapshot(s); err != nil {
		logger.Verbosef("ERROR cosiHandleFinalization ValidateSnapshot %s %v %s\n", m.PeerId, s, err.Error())
		return nil
	}
	chain.AddSnapshot(final, cache, s, signers)
	m.finalized = true
	return chain.node.reloadConsensusNodesList(s, tx)
}

func (node *Node) CosiQueueExternalAnnouncement(peerId crypto.Hash, s *common.Snapshot, commitment *crypto.Key) error {
	logger.Debugf("CosiQueueExternalAnnouncement(%s, %v)\n", peerId, s)
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		logger.Verbosef("CosiQueueExternalAnnouncement(%s, %v) from malicious node\n", peerId, s)
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
	logger.Debugf("CosiAggregateSelfCommitments(%s, %s)\n", peerId, snap)
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		logger.Verbosef("CosiAggregateSelfCommitments(%s, %s) from malicious node\n", peerId, snap)
		return nil
	}

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionSelfCommitment,
		SnapshotHash: snap,
		Commitment:   commitment,
		WantTx:       wantTx,
	}
	node.chain.AppendCosiAction(m)
	return nil
}

func (node *Node) CosiQueueExternalChallenge(peerId crypto.Hash, snap crypto.Hash, cosi *crypto.CosiSignature, ver *common.VersionedTransaction) error {
	logger.Debugf("CosiQueueExternalChallenge(%s, %s)\n", peerId, snap)
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		logger.Verbosef("CosiQueueExternalChallenge(%s, %s) from malicious node\n", peerId, snap)
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
	logger.Debugf("CosiAggregateSelfResponses(%s, %s)\n", peerId, snap)
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		logger.Verbosef("CosiAggregateSelfResponses(%s, %s) from malicious node\n", peerId, snap)
		return nil
	}

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionSelfResponse,
		SnapshotHash: snap,
		Response:     response,
	}
	node.chain.AppendCosiAction(m)
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

	tx, err := node.checkTxInStorage(s.Transaction)
	if err != nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) check tx error %s\n", peerId, s.Hash, err)
	} else if tx == nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) SendTransactionRequestMessage %s\n", peerId, s.Hash, s.Transaction)
		node.Peer.SendTransactionRequestMessage(peerId, s.Transaction)
	}

	chain := node.GetOrCreateChain(s.NodeId)
	if _, finalized := chain.verifyFinalization(s); !finalized {
		logger.Verbosef("ERROR VerifyAndQueueAppendSnapshotFinalization %s %v %d %t %v %v\n", peerId, s, node.ConsensusThreshold(s.Timestamp), chain.IsPledging(), chain.State, chain.ConsensusInfo)
		return nil
	}

	if s.Version == 0 {
		err := chain.legacyAppendFinalization(peerId, s)
		if err != nil {
			logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) legacyAppendFinalization error %s\n", peerId, s.Hash, err)
		}
		return err
	}

	err = chain.AppendFinalSnapshot(peerId, s)
	if err != nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) chain error %s\n", peerId, s.Hash, err)
	}
	return err
}
