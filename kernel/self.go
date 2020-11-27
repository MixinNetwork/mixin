package kernel

import (
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) validateSnapshotTransaction(s *common.Snapshot, finalized bool) (*common.VersionedTransaction, bool, error) {
	tx, snap, err := node.persistStore.ReadTransaction(s.Transaction)
	if err == nil && tx != nil {
		err = node.validateKernelSnapshot(s, tx, finalized)
	}
	if err != nil || tx != nil {
		return tx, len(snap) > 0, err
	}

	tx, err = node.persistStore.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return nil, false, err
	}

	err = tx.Validate(node.persistStore)
	if err != nil {
		return nil, false, err
	}
	err = node.validateKernelSnapshot(s, tx, finalized)
	if err != nil {
		return nil, false, err
	}

	err = tx.LockInputs(node.persistStore, finalized)
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

func (chain *Chain) determinBestRound(roundTime uint64, hint crypto.Hash) (*FinalRound, error) {
	chain.node.chains.RLock()
	defer chain.node.chains.RUnlock()

	chain.State.RLock()
	defer chain.State.RUnlock()

	if chain.State.FinalRound == nil {
		return nil, nil
	}

	var valid bool
	var best *FinalRound
	var start, height uint64
	nodes := chain.node.AcceptedNodesList(roundTime)
	for _, cn := range nodes {
		id := cn.IdForNetwork
		valid = valid || id == hint
		ec, link := chain.node.chains.m[id], chain.State.RoundLinks[id]
		history := historySinceRound(ec.State.RoundHistory, link)
		if len(history) == 0 {
			if id != hint {
				continue
			}
			return nil, fmt.Errorf("external hint history empty since %d", link)
		}

		r, cr := history[0], ec.State.CacheRound
		rts, rh := r.Start, uint64(len(history))
		if id == chain.ChainId || rh < height || rts > roundTime {
			continue
		}

		if !chain.node.genesisNodesMap[id] && r.Number < 7+config.SnapshotReferenceThreshold*2 {
			if id != hint {
				continue
			}
			return nil, fmt.Errorf("external hint round too early yet not genesis %d", r.Number)
		}
		if ts := rts + config.SnapshotRoundGap*rh; ts > uint64(clock.Now().UnixNano()) {
			if id != hint {
				continue
			}
			return nil, fmt.Errorf("external hint round timestamp too future %d %d", ts, clock.Now().UnixNano())
		}
		if len(cr.Snapshots) == 0 && cr.Number == r.Number+1 && r.Number > 0 {
			if id != hint {
				continue
			}
			return nil, fmt.Errorf("external hint round without extra final yet %d", r.Number)
		}

		if rh > height || rts > start {
			best, start, height = r, rts, rh
		}
	}
	if valid {
		return best, nil
	}
	return nil, fmt.Errorf("external hint not found in consensus %s", hint)
}

func historySinceRound(history []*FinalRound, link uint64) []*FinalRound {
	for i, r := range history {
		if r.Number >= link {
			return history[i:]
		}
	}
	return nil
}
