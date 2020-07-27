package kernel

import (
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) checkCacheSnapshotTransaction(s *common.Snapshot) (*common.VersionedTransaction, bool, error) {
	tx, finalized, err := node.persistStore.ReadTransaction(s.Transaction)
	if err == nil && tx != nil {
		err = node.validateKernelSnapshot(s, tx, false)
	}
	if err != nil || tx != nil {
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
	err = node.validateKernelSnapshot(s, tx, false)
	if err != nil {
		return nil, false, err
	}

	err = tx.LockInputs(node.persistStore, false)
	if err != nil {
		return nil, false, err
	}

	return tx, false, node.persistStore.WriteTransaction(tx)
}

func (node *Node) validateKernelSnapshot(s *common.Snapshot, tx *common.VersionedTransaction, finalized bool) error {
	switch tx.TransactionType() {
	case common.TransactionTypeMint:
		err := node.validateMintSnapshot(s, tx)
		if err != nil {
			logger.Verbosef("validateMintSnapshot ERROR %v %s %s\n", s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeNodePledge:
		err := node.validateNodePledgeSnapshot(s, tx)
		if err != nil {
			logger.Verbosef("validateNodePledgeSnapshot ERROR %v %s %s\n", s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeNodeCancel:
		err := node.validateNodeCancelSnapshot(s, tx, finalized)
		if err != nil {
			logger.Verbosef("validateNodeCancelSnapshot ERROR %v %s %s\n", s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeNodeAccept:
		err := node.validateNodeAcceptSnapshot(s, tx, finalized)
		if err != nil {
			logger.Verbosef("validateNodeAcceptSnapshot ERROR %v %s %s\n", s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeNodeRemove:
		err := node.validateNodeRemoveSnapshot(s, tx)
		if err != nil {
			logger.Verbosef("validateNodeRemoveSnapshot ERROR %v %s %s\n", s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	}
	if s.NodeId != node.IdForNetwork && s.RoundNumber == 0 && tx.TransactionType() != common.TransactionTypeNodeAccept {
		return fmt.Errorf("invalid initial transaction type %d", tx.TransactionType())
	}
	return nil
}

func (chain *Chain) determinBestRound(roundTime uint64) *FinalRound {
	chain.node.chains.RLock()
	defer chain.node.chains.RUnlock()

	chain.State.RLock()
	defer chain.State.RUnlock()

	var best *FinalRound
	var start, height uint64
	for id, _ := range chain.node.ConsensusNodes {
		ec := chain.node.chains.m[id]
		history := historySinceRound(ec.State.RoundHistory, chain.State.RoundLinks[id])
		if len(history) == 0 {
			continue
		}
		r := history[0]
		rts, rh := r.Start, uint64(len(history))
		if id == chain.ChainId || rh < height || rts > roundTime {
			continue
		}
		if !chain.node.genesisNodesMap[id] && r.Number < 7+config.SnapshotReferenceThreshold*2 {
			continue
		}
		if rl := chain.State.ReverseRoundLinks[id]; rl >= chain.State.CacheRound.Number {
			continue
		}
		if rts+config.SnapshotRoundGap*rh > uint64(clock.Now().UnixNano()) {
			continue
		}
		if cr := ec.State.CacheRound; len(cr.Snapshots) == 0 && cr.Number == r.Number+1 && r.Number > 0 {
			continue
		}
		if rh > height || rts > start {
			best, start, height = r, rts, rh
		}
	}
	return best
}

func historySinceRound(history []*FinalRound, link uint64) []*FinalRound {
	for i, r := range history {
		if r.Number >= link {
			return history[i:]
		}
	}
	return nil
}
