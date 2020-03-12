package kernel

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) checkCacheSnapshotTransaction(s *common.Snapshot) (*common.VersionedTransaction, bool, error) {
	tx, finalized, err := node.persistStore.ReadTransaction(s.Transaction)
	if tx != nil && len(finalized) == 0 {
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
	case common.TransactionTypeNodeCancel:
		err := node.validateNodeCancelSnapshot(s, tx)
		if err != nil {
			logger.Println("validateNodeCancelSnapshot", s, tx, err)
			return err
		}
	case common.TransactionTypeNodeAccept:
		err := node.validateNodeAcceptSnapshot(s, tx)
		if err != nil {
			logger.Println("validateNodeAcceptSnapshot", s, tx, err)
			return err
		}
	case common.TransactionTypeNodeRemove:
		err := node.validateNodeRemoveSnapshot(s, tx)
		if err != nil {
			logger.Println("validateNodeRemoveSnapshot", s, tx, err)
			return err
		}
	}
	if s.NodeId != node.IdForNetwork && s.RoundNumber == 0 && tx.TransactionType() != common.TransactionTypeNodeAccept {
		return fmt.Errorf("invalid initial transaction type %d", tx.TransactionType())
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
		if rl := node.Graph.ReverseRoundLinks[id]; rl > 0 && rl+1 >= node.Graph.FinalRound[node.IdForNetwork].Number {
			continue
		}
		rts, rh := rounds[0].Start, uint64(len(rounds))
		if id == node.IdForNetwork || rh < height || rts > roundTime {
			continue
		}
		if rts+config.SnapshotRoundGap*rh > uint64(clock.Now().UnixNano()) {
			continue
		}
		if cr := node.Graph.CacheRound[id]; len(cr.Snapshots) == 0 && cr.Number == rounds[0].Number+1 {
			continue
		}
		if rh > height || rts > start {
			best = rounds[0]
			start, height = rts, rh
		}
	}
	return best
}
