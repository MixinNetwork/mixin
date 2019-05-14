package kernel

import (
	"fmt"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) checkCacheSnapshotTransaction(s *common.Snapshot) (*common.VersionedTransaction, error) {
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return nil, err
	}

	finalized, err := node.persistStore.CheckTransactionFinalization(s.Transaction)
	if err != nil || finalized {
		return nil, err
	}

	tx, err := node.persistStore.ReadTransaction(s.Transaction)
	if err != nil || tx != nil {
		return tx, err
	}

	tx, err = node.persistStore.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return nil, err
	}

	err = tx.Validate(node.persistStore)
	if err != nil {
		return nil, nil
	}

	switch tx.TransactionType() {
	case common.TransactionTypeMint:
		err = node.validateMintSnapshot(s, tx)
		if err != nil {
			logger.Println("validateMintSnapshot", s, tx, err)
			return nil, nil
		}
	case common.TransactionTypeNodePledge:
		err = node.validateNodePledgeSnapshot(s, tx)
		if err != nil {
			logger.Println("validateNodePledgeSnapshot", s, tx, err)
			return nil, nil
		}
	case common.TransactionTypeNodeAccept:
		err = node.validateNodeAcceptSnapshot(s, tx)
		if err != nil {
			logger.Println("validateNodeAcceptSnapshot", s, tx, err)
			return nil, nil
		}
	}

	err = tx.LockInputs(node.persistStore, false)
	if err != nil {
		return nil, nil
	}

	if d := tx.DepositData(); d != nil {
		err = node.persistStore.WriteAsset(d.Asset())
		if err != nil {
			return nil, err
		}
	}
	return tx, node.persistStore.WriteTransaction(tx)
}

func (node *Node) collectSelfSignatures(s *common.Snapshot, tx *common.VersionedTransaction) error {
	if s.NodeId != node.IdForNetwork || len(s.Signatures) != 1 {
		panic("should never be here")
	}
	if len(node.SnapshotsPool[s.Hash]) == 0 || node.SignaturesPool[s.Hash] == nil {
		return nil
	}

	filter := make(map[string]bool)
	osigs := node.SnapshotsPool[s.Hash]
	for _, sig := range osigs {
		filter[sig.String()] = true
	}
	for _, sig := range s.Signatures {
		if filter[sig.String()] {
			continue
		}
		osigs = append(osigs, sig)
		filter[sig.String()] = true
	}
	node.SnapshotsPool[s.Hash] = append([]*crypto.Signature{}, osigs...)

	if node.checkInitialAcceptSnapshot(s, tx) {
		if !node.verifyFinalization(osigs) {
			return nil
		}
		s.Signatures = append([]*crypto.Signature{}, osigs...)
		err := node.finalizeNodeAcceptSnapshot(s)
		if err != nil {
			return err
		}
		for peerId, _ := range node.ConsensusNodes {
			err := node.Peer.SendSnapshotMessage(peerId, s, 1)
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
	if !cache.ValidateSnapshot(s, false) {
		return node.clearAndQueueSnapshotOrPanic(s)
	}

	if !node.verifyFinalization(osigs) {
		return nil
	}

	s.Signatures = append([]*crypto.Signature{}, osigs...)
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
	node.Graph.CacheRound[s.NodeId] = cache
	node.removeFromCache(s)

	for peerId, _ := range node.ConsensusNodes {
		err := node.Peer.SendSnapshotMessage(peerId, s, 1)
		if err != nil {
			return err
		}
	}
	return node.reloadConsensusNodesList(s, tx)
}

func (node *Node) determinBestRound(roundTime uint64) *FinalRound {
	var best *FinalRound
	var start, height uint64
	for id, rounds := range node.Graph.RoundHistory {
		if len(rounds) > config.SnapshotReferenceThreshold {
			rc := len(rounds) - config.SnapshotReferenceThreshold
			rounds = append([]*FinalRound{}, rounds[rc:]...)
		}
		node.Graph.RoundHistory[id] = rounds
		rts, rh := rounds[0].Start, uint64(len(rounds))
		if id == node.IdForNetwork || rh < height {
			continue
		}
		if rts > roundTime {
			continue
		}
		if rts+config.SnapshotRoundGap*rh > uint64(time.Now().UnixNano()) {
			continue
		}
		if rh > height || rts > start {
			best = rounds[0]
			start, height = rts, rh
		}
	}
	return best
}

func (node *Node) signSelfSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	if s.NodeId != node.IdForNetwork || len(s.Signatures) != 0 || s.Timestamp != 0 {
		panic("should never be here")
	}
	if node.checkInitialAcceptSnapshot(s, tx) {
		s.Timestamp = uint64(time.Now().UnixNano())
		node.signSnapshot(s)
		s.Signatures = []*crypto.Signature{node.SignaturesPool[s.Hash]}
		for peerId, _ := range node.ConsensusNodes {
			err := node.Peer.SendTransactionMessage(peerId, tx)
			if err != nil {
				return err
			}
			err = node.Peer.SendSnapshotMessage(peerId, s, 0)
			if err != nil {
				return err
			}
		}
		return nil
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if !node.checkCacheCapability() {
		time.Sleep(10 * time.Millisecond)
		return node.queueSnapshotOrPanic(s, false)
	}
	if len(cache.Snapshots) == 0 && !node.CheckBroadcastedToPeers() {
		time.Sleep(time.Duration(config.SnapshotRoundGap / 2))
		return node.queueSnapshotOrPanic(s, false)
	}
	if node.checkCacheExist(s) {
		return nil
	}

	for {
		s.Timestamp = uint64(time.Now().UnixNano())
		if s.Timestamp > cache.Timestamp {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	if start, _ := cache.Gap(); s.Timestamp >= start+config.SnapshotRoundGap {
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
		node.CachePool[s.NodeId] = make([]*common.Snapshot, 0)
	}
	cache.Timestamp = s.Timestamp

	s.RoundNumber = cache.Number
	s.References = cache.References
	node.assignNewGraphRound(final, cache)
	node.signSnapshot(s)
	s.Signatures = []*crypto.Signature{node.SignaturesPool[s.Hash]}
	for peerId, _ := range node.ConsensusNodes {
		err := node.Peer.SendTransactionMessage(peerId, tx)
		if err != nil {
			return err
		}
		err = node.Peer.SendSnapshotMessage(peerId, s, 0)
		if err != nil {
			return err
		}
	}
	node.CachePool[s.NodeId] = append(node.CachePool[s.NodeId], s)
	return nil
}

func (node *Node) checkCacheExist(s *common.Snapshot) bool {
	for _, c := range node.CachePool[s.NodeId] {
		if c.Transaction == s.Transaction {
			return true
		}
	}
	return false
}

func (node *Node) checkCacheCapability() bool {
	pool := node.CachePool[node.IdForNetwork]
	count := len(pool)
	if count == 0 {
		return true
	}
	sort.Slice(pool, func(i, j int) bool {
		return pool[i].Timestamp < pool[j].Timestamp
	})
	start := pool[0].Timestamp
	end := pool[count-1].Timestamp
	if uint64(time.Now().UnixNano()) >= start+config.SnapshotRoundGap*3/2 {
		return true
	}
	return end < start+config.SnapshotRoundGap/3*2
}

func (node *Node) removeFromCache(s *common.Snapshot) {
	pool := node.CachePool[s.NodeId]
	for i, c := range pool {
		if c.Hash != s.Hash {
			continue
		}
		l := len(pool)
		pool[l-1], pool[i] = pool[i], pool[l-1]
		node.CachePool[s.NodeId] = pool[:l-1]
		return
	}
}
