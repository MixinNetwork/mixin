package kernel

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger/v2"
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
		if node.networkId.String() == config.MainnetId && transactionForkHackCheck(tx.PayloadHash()) {
			logger.Printf("transaction fork hack %s\n", tx.PayloadHash())
		} else {
			return nil, false, err
		}
	}
	err = node.validateKernelSnapshot(s, tx, finalized)
	if err != nil {
		return nil, false, err
	}

	err = node.lockAndPersistTransaction(tx, finalized)
	return tx, false, err
}

func (node *Node) lockAndPersistTransaction(tx *common.VersionedTransaction, finalized bool) error {
	for {
		err := tx.LockInputs(node.persistStore, finalized)
		if errors.Is(err, badger.ErrConflict) {
			continue
		} else if err != nil {
			return err
		}

		err = node.persistStore.WriteTransaction(tx)
		if errors.Is(err, badger.ErrConflict) {
			continue
		}
		return err
	}
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
