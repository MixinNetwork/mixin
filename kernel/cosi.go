package kernel

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
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
	CosiActionExternalCommitments
	CosiActionSelfFullCommitment
	CosiActionExternalFullChallenge
)

type CosiAction struct {
	Action       int
	PeerId       crypto.Hash
	SnapshotHash crypto.Hash
	Snapshot     *common.Snapshot
	Commitment   *crypto.Key
	Signature    *crypto.CosiSignature
	Response     *[32]byte
	Transactions []*common.VersionedTransaction
	WantTxs      []crypto.Hash
	Commitments  []*crypto.Key
	Challenge    *crypto.Key
	nonce        *crypto.CosiNonce
	finalized    bool
	data         *CosiChainData
}

type CosiChainData struct {
	PN       *CNode
	CN       *CNode
	FoundTxs map[crypto.Hash]*common.VersionedTransaction
	WantTxs  []crypto.Hash
}

type CosiAggregator struct {
	Snapshot       *common.Snapshot
	Transactions   []*common.VersionedTransaction
	WantTxs        map[crypto.Hash][]crypto.Hash
	FullChallenges map[crypto.Hash]bool
	Commitments    map[int]*crypto.Key
	Responses      map[int]*[32]byte
}

type CosiVerifier struct {
	Snapshot     *common.Snapshot
	Announcement *crypto.Key
	nonce        *crypto.CosiNonce
}

func (node *Node) cosiAcceptedNodesListShuffle(ts uint64) []*CNode {
	var nodes []*CNode // copy the nodes list to avoid conflicts
	nodes = append(nodes, node.NodesListWithoutState(ts, true)...)
	for i := len(nodes) - 1; i > 0; i-- {
		j := int(ts % uint64(i+1))
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	return nodes
}

func (chain *Chain) cosiHook(m *CosiAction) (bool, error) {
	logger.Debugf("cosiHook(%s) %v\n", chain.ChainId, m)
	if !chain.running {
		return false, nil
	}
	err := chain.cosiHandleAction(m)
	if err != nil {
		if m.Action == CosiActionSelfEmpty && shouldRequeueSelfAnnouncement(err) {
			chain.node.requeueTransactions(m.Snapshot.Transactions)
			logger.Debugf("cosiHook requeue %v ERROR %s\n", m, err)
			return false, nil
		}
		return false, err
	}
	if m.Action != CosiActionFinalization {
		return false, nil
	}
	if m.finalized || len(m.WantTxs) == 0 || m.PeerId == chain.node.IdForNetwork {
		return m.finalized, nil
	}
	chain.node.requestTransactionsFromPeer(m.PeerId, m.WantTxs)
	return m.finalized, nil
}

func (node *Node) requestTransactionsFromPeer(peerId crypto.Hash, txs []crypto.Hash) {
	for _, txh := range txs {
		err := node.Peer.SendTransactionRequestMessage(peerId, txh)
		logger.Debugf("requestTransactionsFromPeer(%s, %s) => %v\n", peerId, txh, err)
	}
}

func (chain *Chain) cosiHandleAction(m *CosiAction) error {
	if m.Action == CosiActionFinalization {
		return chain.cosiHandleFinalization(m)
	}
	if m.Action == CosiActionExternalCommitments {
		return chain.cosiAddCommitments(m)
	}
	if err := chain.checkActionSanity(m); err != nil {
		if m.Action == CosiActionSelfEmpty && shouldRequeueSelfAnnouncement(err) {
			chain.node.requeueTransactions(m.Snapshot.Transactions)
		}
		logger.Debugf("cosiHandleAction checkActionSanity %v ERROR %s\n", m, err)
		return nil
	}

	switch m.Action {
	case CosiActionSelfEmpty:
		return chain.cosiSendAnnouncement(m)
	case CosiActionSelfCommitment, CosiActionSelfFullCommitment:
		return chain.cosiHandleCommitment(m)
	case CosiActionSelfResponse:
		return chain.cosiHandleResponse(m)
	case CosiActionExternalAnnouncement:
		return chain.cosiHandleAnnouncement(m)
	case CosiActionExternalChallenge:
		return chain.cosiHandleChallenge(m)
	case CosiActionExternalFullChallenge:
		return chain.cosiHandleFullChallenge(m)
	}

	return nil
}

func shouldRequeueSelfAnnouncement(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "chain not broadcasted") ||
		strings.Contains(msg, "node is slow in catching up") ||
		strings.Contains(msg, "finalized in snapshot")
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
		s.Timestamp = clock.NowUnixNano()
	case CosiActionSelfCommitment, CosiActionSelfFullCommitment, CosiActionSelfResponse:
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
		for _, txh := range s.Transactions {
			ov := chain.CosiVerifiers[txh]
			if ov != nil && s.RoundNumber > 0 && ov.Snapshot.RoundNumber == s.RoundNumber &&
				s.Timestamp < ov.Snapshot.Timestamp+config.SnapshotRoundGap {
				return fmt.Errorf("a transaction %s only in one round %d of one chain %s",
					txh, s.RoundNumber, chain.ChainId)
			}
		}
	case CosiActionExternalFullChallenge:
		if chain.ChainId == chain.node.IdForNetwork {
			return fmt.Errorf("external action announcement chain %s %s", chain.ChainId, chain.node.IdForNetwork)
		}
		if chain.ChainId != m.PeerId {
			return fmt.Errorf("external action announcement peer %s %s", chain.ChainId, m.PeerId)
		}
		if s.Signature != nil || s.Timestamp == 0 || m.Challenge == nil {
			return fmt.Errorf("only empty snapshot with timestamp and challenge can be fully challenged")
		}
		m.nonce = chain.cosiRetrieveRandom(m.SnapshotHash, m.PeerId, m.Challenge)
		if m.nonce == nil {
			err := chain.cosiPrepareRandomsAndSendCommitments(m.PeerId)
			return fmt.Errorf("no match random for the commitment %v %v", m, err)
		}
		for _, txh := range s.Transactions {
			ov := chain.CosiVerifiers[txh]
			if ov != nil && s.RoundNumber > 0 && ov.Snapshot.RoundNumber == s.RoundNumber &&
				s.Timestamp < ov.Snapshot.Timestamp+config.SnapshotRoundGap {
				return fmt.Errorf("a transaction %s only in one round %d of one chain %s",
					txh, s.RoundNumber, chain.ChainId)
			}
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
	if s.Version != common.SnapshotVersionCommonEncoding {
		return fmt.Errorf("invalid snapshot version %d", s.Version)
	}
	if s.NodeId != chain.ChainId {
		return fmt.Errorf("invalid snapshot node id %s %s", s.NodeId, chain.ChainId)
	}

	for _, tx := range m.Transactions {
		err := chain.node.persistStore.CacheStoreTransaction(tx)
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
		if s.Timestamp > clock.NowUnixNano()+threshold {
			return fmt.Errorf("future snapshot timestamp %d", s.Timestamp)
		}
		if s.Timestamp+threshold*2 < chain.node.GraphTimestamp {
			return fmt.Errorf("past snapshot timestamp %d", s.Timestamp)
		}
	}

	if !chain.IsPledging() && !chain.node.CheckCatchUpWithPeers() {
		return fmt.Errorf("node is slow in catching up")
	}

	rn := chain.node.GetRemovingOrSlashingNode(m.PeerId)
	if rn != nil {
		return fmt.Errorf("peer node %s is removing or slashing", m.PeerId)
	}

	cn := chain.node.getAcceptedOrPledgingNode(chain.ChainId, s.Timestamp)
	if cn == nil {
		return fmt.Errorf("chain node %s not found", chain.ChainId)
	}
	pn := chain.node.getAcceptedOrPledgingNode(m.PeerId, s.Timestamp)
	if pn == nil {
		return fmt.Errorf("peer node %s not found", m.PeerId)
	}
	if s.RoundNumber != 0 && !chain.node.ConsensusReady(cn, s.Timestamp) {
		return fmt.Errorf("chain node %s not accepted", cn.IdForNetwork)
	}
	if s.RoundNumber != 0 && !chain.node.ConsensusReady(pn, s.Timestamp) {
		return fmt.Errorf("peer node %s not accepted", pn.IdForNetwork)
	}

	found, missing, err := chain.node.validateSnapshotTransaction(s, false)
	if err != nil {
		return fmt.Errorf("cosi snapshot transaction error %v", err)
	}
	if m.Action != CosiActionExternalAnnouncement && len(found) != len(s.Transactions) {
		return fmt.Errorf("no transaction found %d %d", len(found), len(s.Transactions))
	}

	m.Transactions = slices.Collect(maps.Values(found))
	m.data = &CosiChainData{PN: pn, CN: cn, FoundTxs: found, WantTxs: missing}
	return nil
}

func checkNodeAccept(found map[crypto.Hash]*common.VersionedTransaction) bool {
	txs := slices.Collect(maps.Values(found))
	if !slices.ContainsFunc(txs, func(v *common.VersionedTransaction) bool {
		return v.TransactionType() == common.TransactionTypeNodeAccept
	}) {
		return false
	}
	if len(found) != 1 {
		panic(len(found))
	}
	return true
}

func (chain *Chain) prepareAnnouncement(m *CosiAction) (bool, error) {
	s, cd := m.Snapshot, m.data
	if chain.IsPledging() && s.RoundNumber == 0 && checkNodeAccept(cd.FoundTxs) {
		return true, nil
	}
	if chain.State == nil {
		chain.node.requeueTransactions(s.Transactions)
		return false, nil
	}

	cache, final := chain.StateCopy()
	if len(cache.Snapshots) == 0 && !chain.node.CheckBroadcastedToPeers() {
		chain.node.requeueTransactions(s.Transactions)
		return false, nil
	}
	if s.Timestamp <= cache.Timestamp {
		chain.node.requeueTransactions(s.Transactions)
		return false, nil
	}

	if len(cache.Snapshots) == 0 {
		external, err := chain.persistStore.ReadRound(cache.References.External)
		if err != nil {
			return false, err
		}
		best := chain.determineBestRound(s.Timestamp)
		threshold := external.Timestamp + config.SnapshotReferenceThreshold*config.SnapshotRoundGap*36
		if best != nil && best.NodeId != final.NodeId && threshold < best.Start {
			logger.Verbosef("cosiSendAnnouncement new best external %s:%d:%d => %s:%d:%d\n",
				external.NodeId, external.Number, external.Timestamp, best.NodeId, best.Number, best.Start)
			references := &common.RoundLink{Self: final.Hash, External: best.Hash}
			err := chain.updateEmptyHeadRoundAndPersist(final, cache, references, s.Timestamp, true)
			if err != nil {
				logger.Verbosef("ERROR cosiSendAnnouncement updateEmptyHeadRoundAndPersist failed %s %s %v\n",
					m.PeerId, s.Hash, err)
				return false, nil
			}
			chain.node.requeueTransactions(s.Transactions)
			return false, nil
		}
	} else if start, _ := cache.Gap(); s.Timestamp >= start+config.SnapshotRoundGap {
		best := chain.determineBestRound(s.Timestamp)
		if best == nil {
			logger.Verbosef("cosiSendAnnouncement no best available\n")
			chain.node.requeueTransactions(s.Transactions)
			return false, nil
		}
		if best.NodeId == final.NodeId {
			panic("should never be here")
		}
		references := &common.RoundLink{Self: cache.asFinal().Hash, External: best.Hash}
		nc, nf, _, err := chain.startNewRoundAndPersist(cache, references, s.Timestamp, false)
		if err != nil || nf == nil {
			logger.Verbosef("cosiSendAnnouncement %s %v startNewRoundAndPersist %v %v\n",
				m.PeerId, m.Snapshot, err, nf)
			chain.node.requeueTransactions(s.Transactions)
			return false, nil
		}
		cache, final = nc, nf
		chain.resetCosiStateForNewRound(s.Transactions)
	}
	cache.Timestamp = s.Timestamp
	if final.Number+1 != cache.Number {
		panic(final.Number)
	}

	if len(cache.Snapshots) > 0 {
		cft := cache.Snapshots[0].Timestamp
		if s.Timestamp > cft+uint64(config.SnapshotRoundGap*4/5) {
			logger.Debugf("cosiSendAnnouncement defer %v after current round cutoff %d %d\n",
				s.Transactions, s.Timestamp, cft)
			chain.node.requeueTransactions(s.Transactions)
			return false, nil
		}
		if s.Timestamp/OneDay != cft/OneDay {
			chain.node.requeueTransactions(s.Transactions)
			return false, nil
		}
	}
	s.RoundNumber = cache.Number
	s.References = cache.References
	return true, nil
}

func (chain *Chain) cosiSendAnnouncement(m *CosiAction) error {
	logger.Verbosef("cosiSendAnnouncement %v\n", m.Snapshot)
	valid, err := chain.prepareAnnouncement(m)
	if err != nil || !valid {
		return err
	}

	s, cd := m.Snapshot, m.data
	var duplicate bool
	var retry []crypto.Hash
	for _, txh := range s.Transactions {
		ov := chain.CosiVerifiers[txh]
		if ov != nil && s.RoundNumber > 0 && ov.Snapshot.RoundNumber == s.RoundNumber &&
			s.Timestamp < ov.Snapshot.Timestamp+config.SnapshotRoundGap {
			duplicate = true
			continue
		}
		retry = append(retry, txh)
	}
	if duplicate {
		// Transactions guarded by an existing verifier are already owned by
		// that proposal. Retry only their unguarded batch companions; retrying
		// every transaction here would create a hot duplicate-proposal loop.
		chain.node.requeueTransactions(retry)
		logger.Verbosef("cosiSendAnnouncement defer duplicate transactions in round %d of chain %s\n",
			s.RoundNumber, chain.ChainId)
		return nil
	}
	if len(cd.FoundTxs) != len(s.Transactions) {
		return fmt.Errorf("missing transactions for announcement %d %d", len(cd.FoundTxs), len(s.Transactions))
	}

	s.Hash = s.PayloadHash()
	agg := &CosiAggregator{
		Snapshot:       s,
		Transactions:   slices.Collect(maps.Values(cd.FoundTxs)),
		WantTxs:        make(map[crypto.Hash][]crypto.Hash),
		FullChallenges: make(map[crypto.Hash]bool),
		Commitments:    make(map[int]*crypto.Key),
		Responses:      make(map[int]*[32]byte),
	}

	v := &CosiVerifier{Snapshot: s, nonce: crypto.CosiCommitNonce(crypto.RandReader())}
	R := v.nonce.Public()
	chain.CosiVerifiers[s.Hash] = v
	for _, txh := range s.Transactions {
		chain.CosiVerifiers[txh] = v
	}
	agg.Commitments[cd.CN.ConsensusIndex] = &R
	chain.CosiAggregators[s.Hash] = agg
	nodes := chain.node.cosiAcceptedNodesListShuffle(s.Timestamp)
	for _, cn := range nodes {
		peerId := cn.IdForNetwork
		if peerId == chain.ChainId {
			continue
		}
		commitment := chain.cosiPopCommitment(peerId)
		if commitment == nil || chain.CosiCommunicatedAt[peerId].Before(clock.Now().Add(-time.Duration(config.SnapshotRoundGap)*10)) {
			err := chain.node.Peer.SendSnapshotAnnouncementMessage(peerId, m.Snapshot, R, chain.node.Signer.PrivateSpendKey)
			if err != nil {
				logger.Verbosef("cosiSendAnnouncement SendSnapshotAnnouncementMessage(%s, %s) ERROR %v\n",
					peerId, s.Hash, err)
			}
			continue
		}
		cam := &CosiAction{
			PeerId:       peerId,
			Action:       CosiActionSelfFullCommitment,
			SnapshotHash: s.Hash,
			Commitment:   commitment,
			Snapshot:     s,
		}
		err = chain.AppendCosiAction(cam)
		if err != nil {
			logger.Verbosef("cosiSendAnnouncement AppendCosiAction(%s, %s) ERROR %v\n", peerId, s.Hash, err)
		}
	}
	return nil
}

func (chain *Chain) cosiHandleAnnouncement(m *CosiAction) error {
	logger.Verbosef("cosiHandleAnnouncement %s %v\n", m.PeerId, m.Snapshot)
	valid, err := chain.checkAnnouncementOrChallenge(m)
	if err != nil || !valid {
		return err
	}
	chain.CosiCommunicatedAt[m.PeerId] = clock.Now()

	s, cd := m.Snapshot, m.data
	nonce := crypto.CosiCommitNonce(crypto.RandReader())
	v := &CosiVerifier{Snapshot: s, Announcement: m.Commitment, nonce: nonce}
	chain.CosiVerifiers[s.Hash] = v
	for _, txh := range s.Transactions {
		chain.CosiVerifiers[txh] = v
	}
	err = chain.node.Peer.SendSnapshotCommitmentMessage(s.NodeId, s, nonce.Public(), cd.WantTxs)
	if err != nil {
		logger.Verbosef("cosiHandleAnnouncement SendSnapshotCommitmentMessage(%s, %s) ERROR %v\n",
			s.NodeId, s.Hash, err)
	}
	err = chain.cosiPrepareRandomsAndSendCommitments(s.NodeId)
	if err != nil {
		logger.Verbosef("cosiHandleAnnouncement SendCommitmentsMessage(%s) ERROR %v\n", s.NodeId, err)
	}
	return nil
}

func (chain *Chain) cosiHandleCommitment(m *CosiAction) error {
	logger.Verbosef("cosiHandleCommitment %v\n", m)

	ann := chain.CosiAggregators[m.SnapshotHash]
	if ann == nil {
		logger.Verbosef("cosiHandleCommitment %v EXPIRED\n", m)
		return nil
	}
	s, cd := ann.Snapshot, m.data
	if ann.Commitments[cd.PN.ConsensusIndex] != nil {
		logger.Verbosef("cosiHandleCommitment %v REPEAT\n", m)
		return nil
	}
	base := chain.node.ConsensusThreshold(ann.Snapshot.Timestamp, false)
	if len(ann.Commitments) >= base {
		logger.Verbosef("cosiHandleCommitment %v EXCEED\n", m)
		return nil
	}
	ann.Commitments[cd.PN.ConsensusIndex] = m.Commitment
	ann.WantTxs[m.PeerId] = m.WantTxs
	ann.FullChallenges[m.PeerId] = m.Action == CosiActionSelfFullCommitment
	logger.Verbosef("cosiHandleCommitment %v NOW %d %d\nn", m, len(ann.Commitments), base)
	if len(ann.Commitments) < base {
		return nil
	}
	logger.Verbosef("cosiHandleCommitment %v ENOUGH\n", m)

	cosi, err := crypto.CosiAggregateCommitment(ann.Commitments)
	if err != nil {
		return err
	}
	s.Signature = cosi
	v := chain.CosiVerifiers[m.SnapshotHash]
	priv := chain.node.Signer.PrivateSpendKey
	_, publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	response, err := v.nonce.Response(cosi, &priv, publics, m.SnapshotHash)
	if err != nil { // TODO should slash the malicious node
		logger.Printf("cosiHandleCommitment abandon %s after response failure: %v\n", s.Hash, err)
		chain.retryCosiSnapshot(s)
		return nil
	}
	ann.Responses[cd.CN.ConsensusIndex] = response
	copy(cosi.Signature[32:], response[:])

	nodes := chain.node.cosiAcceptedNodesListShuffle(s.Timestamp)
	for _, cn := range nodes {
		id := cn.IdForNetwork
		txs := slices.Collect(maps.Values(cd.FoundTxs))
		if ann.FullChallenges[id] {
			commitment := ann.Commitments[cd.CN.ConsensusIndex]
			challenge := ann.Commitments[cn.ConsensusIndex]
			if commitment == nil || challenge == nil {
				logger.Printf("cosiHandleCommitment missing full challenge commitment %s %s %d %d\n",
					s.Hash, id, cd.CN.ConsensusIndex, cn.ConsensusIndex)
				continue
			}
			err = chain.node.Peer.SendFullChallengeMessage(id, s, commitment, challenge, txs)
		} else if wantTxs, found := ann.WantTxs[id]; !found {
			continue
		} else {
			var wtxs []*common.VersionedTransaction
			for _, h := range wantTxs {
				tx := cd.FoundTxs[h]
				if tx == nil {
					continue
				}
				wtxs = append(wtxs, tx)
			}
			err = chain.node.Peer.SendTransactionChallengeMessage(id, s, cosi, wtxs)
		}
		if err != nil {
			logger.Verbosef("cosiHandleCommitment SendTransactionChallengeMessage(%s, %s) ERROR %v\n",
				id, m.SnapshotHash, err)
		}
	}
	return nil
}

func (chain *Chain) cosiHandleFullChallenge(m *CosiAction) error {
	logger.Verbosef("cosiHandleFullChallenge %v\n", m)
	if m.nonce == nil {
		panic(m.SnapshotHash)
	}

	valid, err := chain.checkAnnouncementOrChallenge(m)
	if err != nil || !valid {
		return err
	}

	s := m.Snapshot
	v := &CosiVerifier{Snapshot: s, Announcement: m.Commitment, nonce: m.nonce}
	chain.CosiVerifiers[s.Hash] = v
	for _, txh := range s.Transactions {
		chain.CosiVerifiers[txh] = v
	}

	ccm := &CosiAction{
		PeerId:       m.PeerId,
		Action:       CosiActionExternalChallenge,
		SnapshotHash: s.Hash,
		Signature:    m.Signature,
		Transactions: m.Transactions,
	}
	err = chain.AppendCosiAction(ccm)
	if err != nil {
		logger.Verbosef("cosiHandleFullChallenge AppendCosiAction(%s) ERROR %v\n", s.Hash, err)
	}
	return nil
}

func (chain *Chain) checkAnnouncementOrChallenge(m *CosiAction) (bool, error) {
	s := m.Snapshot
	if chain.IsPledging() && s.RoundNumber == 0 {
		return true, nil
	}
	if chain.State == nil {
		logger.Verbosef("checkAnnouncementOrChallenge %s %v empty final round\n", m.PeerId, m.Snapshot)
		return false, nil
	}

	cache, final := chain.StateCopy()
	if s.RoundNumber < cache.Number {
		logger.Verbosef("checkAnnouncementOrChallenge %s %v expired %d %d\n",
			m.PeerId, m.Snapshot, s.RoundNumber, cache.Number)
		return false, nil
	}
	if s.RoundNumber > cache.Number+1 {
		logger.Verbosef("checkAnnouncementOrChallenge %s %v in future %d %d\n",
			m.PeerId, m.Snapshot, s.RoundNumber, cache.Number)
		return false, nil
	}
	if s.Timestamp <= final.Start+config.SnapshotRoundGap {
		logger.Verbosef("checkAnnouncementOrChallenge %s %v invalid timestamp %d %d\n",
			m.PeerId, m.Snapshot, s.Timestamp, final.Start+config.SnapshotRoundGap)
		return false, nil
	}
	if s.RoundNumber == cache.Number && !s.References.Equal(cache.References) {
		err := chain.updateEmptyHeadRoundAndPersist(final, cache, s.References, s.Timestamp, true)
		if err != nil {
			logger.Verbosef("checkAnnouncementOrChallenge %s %v updateEmptyHeadRoundAndPersist %v\n",
				m.PeerId, m.Snapshot, err)
			return false, nil
		}
		return false, chain.AppendCosiAction(m)
	}
	if s.RoundNumber == cache.Number+1 {
		nc, nf, _, err := chain.startNewRoundAndPersist(cache, s.References, s.Timestamp, false)
		if err != nil {
			logger.Verbosef("checkAnnouncementOrChallenge %s %v startNewRoundAndPersist %s\n",
				m.PeerId, m.Snapshot, err)
			return false, chain.AppendCosiAction(m)
		} else if nf == nil {
			logger.Verbosef("checkAnnouncementOrChallenge %s %v startNewRoundAndPersist failed\n",
				m.PeerId, m.Snapshot)
			return false, nil
		}
		cache, final = nc, nf
		chain.CosiVerifiers = make(map[crypto.Hash]*CosiVerifier)
	}
	if final.Number+1 != cache.Number {
		panic(final.Number)
	}

	if err := cache.ValidateSnapshot(s); err != nil {
		logger.Verbosef("checkAnnouncementOrChallenge %s %v ValidateSnapshot %s\n",
			m.PeerId, m.Snapshot, err)
		return false, nil
	}
	return true, nil
}

func (chain *Chain) cosiHandleChallenge(m *CosiAction) error {
	logger.Verbosef("cosiHandleChallenge %v\n", m)
	v := chain.CosiVerifiers[m.SnapshotHash]
	s, cd := v.Snapshot, m.data

	var sig crypto.Signature
	copy(sig[:], v.Announcement[:])
	copy(sig[32:], m.Signature.Signature[32:])
	pub := cd.CN.Signer.PublicSpendKey
	_, publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	challenge, err := m.Signature.Challenge(publics, m.SnapshotHash)
	if err != nil {
		logger.Verbosef("cosiHandleChallenge %v Challenge ERROR %s\n", m, err)
		return nil
	}
	if !pub.VerifyWithChallenge(sig, challenge) {
		logger.Verbosef("cosiHandleChallenge %v VerifyWithChallenge ERROR %v %v\n",
			m, sig, challenge)
		return nil
	}
	chain.CosiCommunicatedAt[m.PeerId] = clock.Now()

	priv := chain.node.Signer.PrivateSpendKey
	response, err := v.nonce.Response(m.Signature, &priv, publics, m.SnapshotHash)
	if errors.Is(err, crypto.ErrCosiNonceReuse) { // TODO should slash the malicious node
		logger.Printf("SECURITY cosiHandleChallenge rejected nonce reuse from %s for snapshot %s\n",
			m.PeerId, m.SnapshotHash)
		chain.abandonCosiSnapshot(s)
		return nil
	}
	if err != nil {
		logger.Verbosef("cosiHandleChallenge %v Response ERROR %s\n", m, err)
		chain.abandonCosiSnapshot(s)
		return nil
	}
	err = chain.node.Peer.SendSnapshotResponseMessage(m.PeerId, m.SnapshotHash, response)
	if err != nil {
		logger.Verbosef("cosiHandleChallenge SendSnapshotResponseMessage(%s, %s) ERROR %v\n",
			m.PeerId, m.SnapshotHash, err)
	}
	return nil
}

func (chain *Chain) cosiHandleResponse(m *CosiAction) error {
	logger.Verbosef("cosiHandleResponse %v\n", m)
	agg := chain.CosiAggregators[m.SnapshotHash]
	s, cd := agg.Snapshot, m.data
	if agg.Responses[cd.PN.ConsensusIndex] != nil {
		logger.Verbosef("cosiHandleResponse %v REPEAT\n", m)
		return nil
	}
	chain.CosiCommunicatedAt[m.PeerId] = clock.Now()
	if len(agg.Responses) >= len(agg.Commitments) {
		logger.Verbosef("cosiHandleResponse %v EXCEED\n", m)
		return nil
	}
	cids, publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	err := s.Signature.VerifyResponse(publics, cd.PN.ConsensusIndex, m.Response, m.SnapshotHash)
	if err != nil {
		logger.Verbosef("cosiHandleResponse %v RESPONSE ERROR %s\n", m, err)
		return nil
	}

	base := chain.node.ConsensusThreshold(s.Timestamp, false)
	agg.Responses[cd.PN.ConsensusIndex] = m.Response
	logger.Verbosef("cosiHandleResponse %v NOW %d %d %d\n",
		m, len(agg.Responses), len(agg.Commitments), base)
	if len(agg.Responses) != len(agg.Commitments) {
		return nil
	}
	logger.Verbosef("cosiHandleResponse %v ENOUGH\n", m)

	err = s.Signature.AggregateResponse(publics, agg.Responses, m.SnapshotHash, false)
	if err != nil {
		logger.Printf("cosiHandleResponse abandon %s after aggregate failure: %v\n", s.Hash, err)
		chain.retryCosiSnapshot(s)
		return nil
	}
	signers, finalized := chain.node.cacheVerifyCosi(m.SnapshotHash, s.Signature, cids, publics, base)
	if !finalized {
		logger.Verbosef("cosiHandleResponse %v AGGREGATE ERROR\n", m)
		chain.retryCosiSnapshot(s)
		return nil
	}
	logger.Verbosef("node.cacheVerifyCosi(%s, %s) FINAL\n", chain.node.Peer.Address, m.SnapshotHash)

	if chain.IsPledging() && s.RoundNumber == 0 && checkNodeAccept(cd.FoundTxs) {
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
			logger.Verbosef("cosiHandleResponse %v EXPIRE %d %d\n",
				m, s.RoundNumber, cache.Number)
			chain.retryCosiSnapshot(s)
			return nil
		}
		if !s.References.Equal(cache.References) {
			logger.Verbosef("cosiHandleResponse %v REFERENCES %v %v\n",
				m, s.References, cache.References)
			chain.retryCosiSnapshot(s)
			return nil
		}
		if err := cache.ValidateSnapshot(s); err != nil {
			logger.Verbosef("cosiHandleResponse %v ValidateSnapshot %s\n", m, err)
			chain.retryCosiSnapshot(s)
			return nil
		}

		err = chain.AddSnapshot(final, cache, s, signers)
		if err != nil {
			panic(err)
		}
	}

	nodes := chain.node.cosiAcceptedNodesListShuffle(s.Timestamp)
	for _, cn := range nodes {
		id := cn.IdForNetwork
		if agg.Responses[cn.ConsensusIndex] == nil {
			err := chain.node.SendTransactionsToPeer(id, s.Transactions, true)
			if err != nil {
				logger.Verbosef("cosiHandleResponse SendFinalizedTransactionsToPeer(%s, %s) ERROR %v\n",
					id, m.SnapshotHash, err)
			}
		}
		err := chain.node.Peer.SendSnapshotFinalizationMessage(id, s)
		if err != nil {
			logger.Verbosef("cosiHandleResponse SendSnapshotFinalizationMessage(%s, %s) ERROR %v\n",
				id, m.SnapshotHash, err)
		}
	}
	if len(cd.FoundTxs) != 1 {
		return nil
	}
	tx := slices.Collect(maps.Values(cd.FoundTxs))[0]
	return chain.node.reloadConsensusState(s, tx)
}

func (chain *Chain) abandonCosiSnapshot(s *common.Snapshot) {
	delete(chain.CosiAggregators, s.Hash)
	verifier := chain.CosiVerifiers[s.Hash]
	delete(chain.CosiVerifiers, s.Hash)
	for _, tx := range s.Transactions {
		if chain.CosiVerifiers[tx] == verifier {
			delete(chain.CosiVerifiers, tx)
		}
	}
}

// retryCosiSnapshot is for terminal failures of a locally aggregated proposal.
// Removing the aggregator first prevents late commitments or responses from
// reviving it, while CacheRequeueTransaction makes every still-unfinalized
// transaction immediately eligible for another owner/proposal.
func (chain *Chain) retryCosiSnapshot(s *common.Snapshot) {
	chain.abandonCosiSnapshot(s)
	chain.node.requeueTransactions(s.Transactions)
}

// resetCosiStateForNewRound retires old-round proposals that cannot complete
// after a local round transition. Clearing the CoSi maps alone would lose their
// transactions because those transactions have already been removed from the
// cache queue, and the deleted aggregators could no longer expire and requeue
// them.
//
// owned contains the transactions in the current snapshot that triggered the
// transition. They are deliberately excluded from retry: after this method
// returns, cosiSendAnnouncement installs a new-round aggregator that owns them.
// Requeueing them here would leave the same transaction both in that active
// aggregator and in the cache queue, allowing a redundant proposal. Any current
// snapshot rejected before its aggregator is installed is requeued explicitly
// by the caller; once installed, terminal-error and expiry paths requeue it.
//
// Transactions belonging only to discarded old-round aggregators are orphaned,
// so this method deduplicates and requeues them before clearing all round-local
// aggregator and verifier state.
func (chain *Chain) resetCosiStateForNewRound(owned []crypto.Hash) {
	keep := make(map[crypto.Hash]bool, len(owned))
	for _, tx := range owned {
		keep[tx] = true
	}

	seen := make(map[crypto.Hash]bool)
	var retry []crypto.Hash
	for _, agg := range chain.CosiAggregators {
		for _, tx := range agg.Snapshot.Transactions {
			if !keep[tx] && !seen[tx] {
				seen[tx] = true
				retry = append(retry, tx)
			}
		}
	}
	chain.CosiAggregators = make(map[crypto.Hash]*CosiAggregator)
	chain.CosiVerifiers = make(map[crypto.Hash]*CosiVerifier)
	chain.node.requeueTransactions(retry)
}

// expireCosiAggregators retries transactions from local snapshot proposals that
// did not collect enough commitments or responses. It is called by
// QueuePollSnapshots, which also processes all CoSi actions, so the aggregator
// and verifier maps are inspected and modified by the same serialized loop.
//
// A self announcement gets its snapshot timestamp immediately before the
// aggregator is created. SnapshotRoundGap is also the period during which a
// verifier rejects another snapshot containing the same transaction, so
// retrying before that gap expires would only cause the next proposal to be
// rejected by peers.
//
// Completed aggregators remain in the map for the existing duplicate guards.
// For an incomplete aggregator, abandonCosiSnapshot removes only the verifier
// entries belonging to that proposal, and requeueTransactions skips already
// finalized transactions before making the others eligible and waking the
// cache queue.
func (chain *Chain) expireCosiAggregators(now uint64) {
	for _, agg := range chain.CosiAggregators {
		s := agg.Snapshot
		if now < s.Timestamp+config.SnapshotRoundGap {
			continue
		}
		base := chain.node.ConsensusThreshold(s.Timestamp, false)
		// Once the commitment threshold and every corresponding response have
		// arrived, cosiHandleResponse owns finalization and this proposal must
		// not be requeued by the timeout path.
		if len(agg.Commitments) >= base && len(agg.Responses) == len(agg.Commitments) {
			continue
		}
		logger.Debugf("expireCosiAggregators requeue %s after %d commitments and %d responses\n",
			s.Hash, len(agg.Commitments), len(agg.Responses))
		chain.retryCosiSnapshot(s)
	}
}

func (chain *Chain) prepareFinalization(m *CosiAction) (bool, error) {
	s := m.Snapshot
	if chain.IsPledging() && s.RoundNumber == 0 {
		return true, nil
	}
	if chain.State == nil {
		logger.Debugf("ERROR cosiHandleFinalization without consensus%s %s\n", m.PeerId, s.Hash)
		return false, nil
	}
	cache := chain.State.CacheRound
	if s.RoundNumber < cache.Number {
		logger.Debugf("ERROR cosiHandleFinalization expired round %s %s %d %d\n",
			m.PeerId, s.Hash, s.RoundNumber, cache.Number)
		return false, nil
	}
	if s.RoundNumber > cache.Number+1 {
		logger.Debugf("ERROR cosiHandleFinalization in future %s %s %d %d\n",
			m.PeerId, s.Hash, s.RoundNumber, cache.Number)
		return false, nil
	}
	if s.RoundNumber == cache.Number+1 {
		_, nf, dummy, err := chain.startNewRoundAndPersist(cache, s.References, s.Timestamp, true)
		if err != nil || nf == nil {
			logger.Verbosef("ERROR cosiHandleFinalization startNewRound %s %v %v %v\n",
				m.PeerId, s, err, nf)
			return false, nil
		}
		if dummy {
			logger.Verbosef("ERROR cosiHandleFinalization startNewRound DUMMY %s %s %d\n",
				m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp, true))
			return false, nil
		}
	}
	return true, nil
}

func (chain *Chain) cosiHandleFinalization(m *CosiAction) error {
	logger.Debugf("cosiHandleFinalization %s %v\n", m.PeerId, m.Snapshot)
	valid, err := chain.prepareFinalization(m)
	if err != nil || !valid {
		return err
	}

	s := m.Snapshot
	signers, finalized := chain.verifyFinalization(s)
	if !finalized {
		logger.Verbosef("ERROR cosiHandleFinalization verifyFinalization %s %v %d\n",
			m.PeerId, s, chain.node.ConsensusThreshold(s.Timestamp, true))
		return nil
	}

	found, missing, err := chain.node.validateSnapshotTransaction(s, true)
	if err != nil {
		logger.Verbosef("ERROR handleFinalization validateSnapshotTransaction %s %s %d %v\n",
			m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp, true), err)
		return nil
	} else if len(missing) > 0 {
		logger.Verbosef("ERROR handleFinalization validateSnapshotTransaction %s %s %d %s\n",
			m.PeerId, s.Hash, chain.node.ConsensusThreshold(s.Timestamp, true), "tx empty")
		m.WantTxs = missing
		return nil
	}

	if chain.IsPledging() && s.RoundNumber == 0 && checkNodeAccept(found) {
		err := chain.node.finalizeNodeAcceptSnapshot(s, signers)
		if err != nil {
			return err
		}
		tx := slices.Collect(maps.Values(found))[0]
		return chain.node.reloadConsensusState(s, tx)
	} else if chain.State == nil {
		return nil
	}

	cache, final := chain.StateCopy()
	if !s.References.Equal(cache.References) {
		err := chain.updateEmptyHeadRoundAndPersist(final, cache, s.References, s.Timestamp, false)
		if err != nil {
			logger.Debugf("ERROR cosiHandleFinalization updateEmptyHeadRoundAndPersist failed %s %s %v\n",
				m.PeerId, s.Hash, err)
		}
		return nil
	}

	if err := cache.ValidateSnapshot(s); err != nil {
		logger.Verbosef("ERROR cosiHandleFinalization ValidateSnapshot %s %v %v\n", m.PeerId, s, err)
		return nil
	}
	err = chain.AddSnapshot(final, cache, s, signers)
	if err != nil {
		panic(err)
	}
	m.finalized = true
	if len(found) > 1 {
		return nil
	}
	tx := slices.Collect(maps.Values(found))[0]
	return chain.node.reloadConsensusState(s, tx)
}

func (chain *Chain) cosiPopCommitment(peerId crypto.Hash) *crypto.Key {
	if chain.ChainId != chain.node.IdForNetwork {
		panic(chain.ChainId)
	}
	if chain.ChainId == peerId {
		panic(peerId)
	}
	commitments := chain.CosiCommitments[peerId]
	if len(commitments) == 0 {
		return nil
	}
	commitment := commitments[0]
	chain.CosiCommitments[peerId] = commitments[1:]
	chain.markCosiCommitmentUsed(*commitment)
	return commitment
}

func (chain *Chain) markCosiCommitmentUsed(commitment crypto.Key) {
	const maximumRetainedCommitments = 1024 * 1024
	if chain.UsedCommitments == nil {
		chain.UsedCommitments = make(map[crypto.Key]bool)
	}
	if chain.UsedCommitments[commitment] {
		return
	}
	chain.UsedCommitments[commitment] = true
	chain.usedCommitmentsOrder = append(chain.usedCommitmentsOrder, commitment)
	if len(chain.usedCommitmentsOrder) <= maximumRetainedCommitments {
		return
	}
	oldest := chain.usedCommitmentsOrder[0]
	chain.usedCommitmentsOrder = chain.usedCommitmentsOrder[1:]
	delete(chain.UsedCommitments, oldest)
}

func (chain *Chain) cosiAddCommitments(m *CosiAction) error {
	if chain.ChainId != chain.node.IdForNetwork {
		panic(chain.ChainId)
	}
	if chain.ChainId == m.PeerId {
		panic(m.PeerId)
	}
	if rn := chain.node.GetRemovingOrSlashingNode(m.PeerId); rn != nil {
		return nil
	}
	chain.CosiCommunicatedAt[m.PeerId] = clock.Now()
	var commitments []*crypto.Key
	for _, k := range m.Commitments {
		if !chain.UsedCommitments[*k] {
			commitments = append(commitments, k)
		}
	}
	logger.Verbosef("cosiAddCommitments(%s, %d) => %d %d",
		m.PeerId, len(m.Commitments), len(commitments), len(chain.UsedCommitments))
	chain.CosiCommitments[m.PeerId] = commitments
	return nil
}

func (chain *Chain) cosiRetrieveRandom(snap crypto.Hash, peerId crypto.Hash, challenge *crypto.Key) *crypto.CosiNonce {
	if chain.ChainId == chain.node.IdForNetwork {
		panic(chain.ChainId)
	}
	if chain.ChainId != peerId {
		panic(peerId)
	}
	nonce := chain.UsedRandoms[snap]
	if nonce != nil && nonce.Public() == *challenge {
		return nonce
	}
	cm := chain.CosiRandoms
	if cm == nil {
		return nil
	}
	nonce = cm[*challenge]
	if nonce == nil {
		return nil
	}
	chain.retainUsedCosiNonce(snap, nonce)
	delete(chain.CosiRandoms, *challenge)
	return nonce
}

func (chain *Chain) retainUsedCosiNonce(snapshot crypto.Hash, nonce *crypto.CosiNonce) {
	const maximumRetainedNonces = 1024 * 128
	if chain.UsedRandoms == nil {
		chain.UsedRandoms = make(map[crypto.Hash]*crypto.CosiNonce)
	}
	if chain.UsedRandoms[snapshot] == nil {
		chain.usedRandomsOrder = append(chain.usedRandomsOrder, snapshot)
	}
	chain.UsedRandoms[snapshot] = nonce
	if len(chain.usedRandomsOrder) <= maximumRetainedNonces {
		return
	}
	oldest := chain.usedRandomsOrder[0]
	chain.usedRandomsOrder = chain.usedRandomsOrder[1:]
	delete(chain.UsedRandoms, oldest)
}

func (chain *Chain) cosiPrepareRandomsAndSendCommitments(peerId crypto.Hash) error {
	const maximum = 512
	if chain.ChainId == chain.node.IdForNetwork {
		panic(chain.ChainId)
	}
	if chain.ChainId != peerId {
		panic(peerId)
	}

	last := chain.CommitmentsSentTime.Add(time.Duration(config.SnapshotRoundGap) * 10)
	if last.After(clock.Now()) && len(chain.CosiRandoms) > maximum/2 {
		return nil
	}

	// FIXME always generate new randoms, may bloat the memory
	commitments := make([]*crypto.Key, maximum)
	if chain.CosiRandoms == nil {
		chain.CosiRandoms = make(map[crypto.Key]*crypto.CosiNonce, maximum)
	}
	for i := range maximum {
		nonce := crypto.CosiCommitNonce(crypto.RandReader())
		k := nonce.Public()
		commitments[i] = &k
		chain.CosiRandoms[k] = nonce
	}
	chain.CommitmentsSentTime = clock.Now()
	return chain.node.Peer.SendCommitmentsMessage(peerId, commitments)
}

func (node *Node) CosiQueueExternalPreCommitments(peerId crypto.Hash, commitments []*crypto.Key, data []byte, sig *crypto.Signature) error {
	logger.Debugf("CosiQueueExternalPreCommitments(%s, %d)\n", peerId, len(commitments))
	peer := node.GetAcceptedOrPledgingNode(peerId)
	if peer == nil {
		logger.Verbosef("CosiQueueExternalPreCommitments(%s, %d) from malicious node\n",
			peerId, len(commitments))
		return nil
	}
	if !peer.Signer.PublicSpendKey.Verify(crypto.Blake3Hash(data), *sig) {
		logger.Printf("CosiQueueExternalPreCommitments(%s) invalid signature\n", peerId)
		return nil
	}

	m := &CosiAction{
		PeerId:      peerId,
		Action:      CosiActionExternalCommitments,
		Commitments: commitments,
	}
	err := node.chain.AppendCosiAction(m)
	if err != nil {
		logger.Verbosef("CosiQueueExternalPreCommitments(%v) => %v\n", m, err)
	}
	return nil
}

func (node *Node) CosiQueueExternalAnnouncement(peerId crypto.Hash, s *common.Snapshot, commitment *crypto.Key, sig *crypto.Signature) error {
	logger.Debugf("CosiQueueExternalAnnouncement(%s, %v)\n", peerId, s)
	peer := node.GetAcceptedOrPledgingNode(peerId)
	if peer == nil {
		logger.Verbosef("CosiQueueExternalAnnouncement(%s, %v) from malicious node\n", peerId, s)
		return nil
	}
	data := append(commitment[:], s.VersionedMarshal()...)
	if !peer.Signer.PublicSpendKey.Verify(crypto.Blake3Hash(data), *sig) {
		logger.Printf("CosiQueueExternalAnnouncement(%s, %v) invalid signature\n", peerId, s)
		return nil
	}
	chain := node.getOrCreateChain(s.NodeId)
	if chain == nil {
		logger.Verbosef("CosiQueueExternalAnnouncement(%s, %v) from malicious node\n", peerId, s)
		return nil
	}

	s.Hash = s.PayloadHash()
	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionExternalAnnouncement,
		Snapshot:     s,
		Commitment:   commitment,
		SnapshotHash: s.Hash,
	}
	err := chain.AppendCosiAction(m)
	if err != nil {
		logger.Verbosef("CosiQueueExternalAnnouncement(%v) => %v\n", m, err)
	}
	return nil
}

func (node *Node) CosiAggregateSelfCommitments(peerId crypto.Hash, snap crypto.Hash, commitment *crypto.Key, wantTxs []crypto.Hash, data []byte, sig *crypto.Signature) error {
	logger.Debugf("CosiAggregateSelfCommitments(%s, %s)\n", peerId, snap)
	peer := node.GetAcceptedOrPledgingNode(peerId)
	if peer == nil {
		logger.Verbosef("CosiAggregateSelfCommitments(%s, %s) from malicious node\n", peerId, snap)
		return nil
	}
	if !peer.Signer.PublicSpendKey.Verify(crypto.Blake3Hash(data), *sig) {
		logger.Printf("CosiAggregateSelfCommitments(%s, %s) invalid signature\n", peerId, snap)
		return nil
	}
	wantTxs = node.resolveLegacyWantTxs(snap, wantTxs)

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionSelfCommitment,
		SnapshotHash: snap,
		Commitment:   commitment,
		WantTxs:      wantTxs,
	}
	err := node.chain.AppendCosiAction(m)
	if err != nil {
		logger.Verbosef("CosiAggregateSelfCommitments(%v) => %v\n", m, err)
	}
	return nil
}

func (node *Node) resolveLegacyWantTxs(snap crypto.Hash, wantTxs []crypto.Hash) []crypto.Hash {
	if wantTxs == nil || len(wantTxs) > 0 {
		return wantTxs
	}
	if node.chain == nil {
		return nil
	}
	agg := node.chain.CosiAggregators[snap]
	if agg == nil || agg.Snapshot == nil || len(agg.Snapshot.Transactions) != 1 {
		return nil
	}
	return []crypto.Hash{agg.Snapshot.Transactions[0]}
}

func (node *Node) CosiQueueExternalChallenge(peerId crypto.Hash, snap crypto.Hash, cosi *crypto.CosiSignature, txs []*common.VersionedTransaction) error {
	logger.Debugf("CosiQueueExternalChallenge(%s, %s)\n", peerId, snap)
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		logger.Verbosef("CosiQueueExternalChallenge(%s, %s) from malicious node\n", peerId, snap)
		return nil
	}
	chain := node.getOrCreateChain(peerId)

	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionExternalChallenge,
		SnapshotHash: snap,
		Signature:    cosi,
		Transactions: txs,
	}
	err := chain.AppendCosiAction(m)
	if err != nil {
		logger.Verbosef("CosiQueueExternalChallenge(%v) => %v\n", m, err)
	}
	return nil
}

func (node *Node) CosiQueueExternalFullChallenge(peerId crypto.Hash, s *common.Snapshot, commitment, challenge *crypto.Key, cosi *crypto.CosiSignature, txs []*common.VersionedTransaction) error {
	logger.Debugf("CosiQueueExternalFullChallenge(%s, %v)\n", peerId, s)
	if node.GetAcceptedOrPledgingNode(peerId) == nil {
		logger.Verbosef("CosiQueueExternalFullChallenge(%s, %v) from malicious node\n", peerId, s)
		return nil
	}
	chain := node.getOrCreateChain(peerId)

	s.Hash = s.PayloadHash()
	m := &CosiAction{
		PeerId:       peerId,
		Action:       CosiActionExternalFullChallenge,
		Snapshot:     s,
		SnapshotHash: s.Hash,
		Commitment:   commitment,
		Challenge:    challenge,
		Signature:    cosi,
		Transactions: txs,
	}
	err := chain.AppendCosiAction(m)
	if err != nil {
		logger.Verbosef("CosiQueueExternalFullChallenge(%v) => %v\n", m, err)
	}
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
	err := node.chain.AppendCosiAction(m)
	if err != nil {
		logger.Verbosef("CosiAggregateSelfResponses(%v) => %v\n", m, err)
	}
	return nil
}

func (node *Node) VerifyAndQueueAppendSnapshotFinalization(peerId crypto.Hash, s *common.Snapshot) error {
	s.Hash = s.PayloadHash()
	logger.Debugf("VerifyAndQueueAppendSnapshotFinalization(%s, %s)\n", peerId, s.Hash)

	node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash)
	err := node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash)
	if err != nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) SendSnapshotConfirmMessage error %s\n",
			peerId, s.Hash, err)
		return nil
	}

	var missing []crypto.Hash
	for _, txh := range s.Transactions {
		tx, finalized, err := node.checkTxInStorage(txh)
		if err != nil {
			logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) check tx error %s\n", peerId, s.Hash, err)
		} else if tx == nil {
			missing = append(missing, txh)
		} else if finalized == s.Hash.String() {
			return nil
		}
	}
	if len(missing) > 0 {
		node.requestTransactionsFromPeer(peerId, missing)
	}

	chain := node.getOrCreateChain(s.NodeId)
	if chain == nil {
		return nil
	}
	if cs := chain.State; cs != nil && cs.CacheRound.index.Check(s.Hash) {
		return nil
	}
	if _, finalized := chain.verifyFinalization(s); !finalized {
		logger.Verbosef("ERROR VerifyAndQueueAppendSnapshotFinalization %s %v %d %t %v %v\n",
			peerId, s, node.ConsensusThreshold(s.Timestamp, true), chain.IsPledging(), chain.State, chain.ConsensusInfo)
		return nil
	}

	err = chain.AppendFinalSnapshot(s.NodeId, s)
	if err != nil {
		logger.Verbosef("VerifyAndQueueAppendSnapshotFinalization(%s, %s) chain error %s\n",
			peerId, s.Hash, err)
	}
	return nil
}
