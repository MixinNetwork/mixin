package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) checkCacheSnapshotTransaction(s *common.Snapshot) (*common.VersionedTransaction, bool, error) {
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return nil, inNode, err
	}

	tx, finalized, err := node.persistStore.ReadTransaction(s.Transaction)
	if tx != nil {
		err = node.validateKernelSnapshot(s, tx)
	}
	if err != nil || len(finalized) > 0 || tx != nil {
		return tx, len(finalized) > 0, err
	}

	tx, err = node.persistStore.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return nil, false, err
	}

	err = tx.Validate(node.persistStore)
	if err != nil {
		return nil, false, err
	}
	err = node.validateKernelSnapshot(s, tx)
	if err != nil {
		return nil, false, err
	}

	err = tx.LockInputs(node.persistStore, false)
	if err != nil {
		return nil, false, err
	}

	return tx, false, node.persistStore.WriteTransaction(tx)
}

func (node *Node) validateKernelSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
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
