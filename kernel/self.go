package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) doSnapshotValidation(s *common.Snapshot, tx *common.VersionedTransaction) error {
	switch tx.TransactionType() {
	case common.TransactionTypeMint:
		err := node.validateMintSnapshot(s, tx)
		if err != nil {
			logger.Println("validateMintSnapshot", s, tx, err)
			return err
		}
	case common.TransactionTypeNodePledge:
		err := node.validateNodePledgeSnapshot(s, tx)
		if err != nil {
			logger.Println("validateNodePledgeSnapshot", s, tx, err)
			return err
		}
	case common.TransactionTypeNodeAccept:
		err := node.validateNodeAcceptSnapshot(s, tx)
		if err != nil {
			logger.Println("validateNodeAcceptSnapshot", s, tx, err)
			return err
		}
	}
	return nil
}

func (node *Node) checkTransaction(nodeId, hash crypto.Hash) (*common.VersionedTransaction, bool, error) {
	inNode, err := node.persistStore.CheckTransactionInNode(nodeId, hash)
	if err != nil || inNode {
		return nil, inNode, err
	}
	tx, finalized, err := node.persistStore.ReadTransaction(hash)
	if err != nil || len(finalized) > 0 || tx != nil {
		return tx, len(finalized) > 0, err
	}
	tx, err = node.persistStore.CacheGetTransaction(hash)
	return tx, false, err
}

func (node *Node) checkCacheSnapshotTransaction(s *common.Snapshot) (*common.VersionedTransaction, error) {
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return nil, err
	}

	tx, finalized, err := node.persistStore.ReadTransaction(s.Transaction)
	if err != nil || len(finalized) > 0 {
		return nil, err
	}
	if tx != nil {
		err = node.doSnapshotValidation(s, tx)
		if err != nil {
			return nil, nil
		}
		return tx, nil
	}

	tx, err = node.persistStore.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return nil, err
	}

	err = tx.Validate(node.persistStore)
	if err != nil {
		return nil, nil
	}
	err = node.doSnapshotValidation(s, tx)
	if err != nil {
		return nil, nil
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
		if !node.verifyFinalization(s.Timestamp, osigs) {
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
	if !s.References.Equal(cache.References) {
		return node.clearAndQueueSnapshotOrPanic(s)
	}
	if !cache.ValidateSnapshot(s, false) {
		return node.clearAndQueueSnapshotOrPanic(s)
	}

	if !node.verifyFinalization(s.Timestamp, osigs) {
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
		if !node.genesisNodesMap[id] && rounds[0].Number < 7+config.SnapshotReferenceThreshold*2 {
			continue
		}
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
