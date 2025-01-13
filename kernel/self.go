package kernel

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger/v4"
)

func (node *Node) checkTxInStorage(id crypto.Hash) (*common.VersionedTransaction, string, error) {
	tx, snap, err := node.persistStore.ReadTransaction(id)
	if err != nil || tx != nil {
		return tx, snap, err
	}

	tx, err = node.persistStore.CacheGetTransaction(id)
	return tx, "", err
}

func (node *Node) validateSnapshotTransaction(s *common.Snapshot, finalized bool) (*common.VersionedTransaction, bool, error) {
	tx, snap, err := node.persistStore.ReadTransaction(s.SoleTransaction())
	if err == nil && tx != nil {
		err = node.validateKernelSnapshot(s, tx, finalized)
	}
	if err != nil || tx != nil {
		return tx, len(snap) > 0, err
	}

	tx, err = node.persistStore.CacheGetTransaction(s.SoleTransaction())
	if err != nil || tx == nil {
		return nil, false, err
	}

	err = tx.Validate(node.persistStore, s.Timestamp, finalized)
	if err != nil {
		return nil, false, err
	}
	err = node.validateKernelSnapshot(s, tx, finalized)
	if err != nil {
		return nil, false, err
	}

	err = node.lockAndPersistTransaction(tx, finalized)
	return tx, false, err
}

func (node *Node) lockAndPersistTransaction(tx *common.VersionedTransaction, finalized bool) error {
	for i := time.Duration(0); i < time.Second; i += time.Millisecond * 100 {
		err := tx.LockInputs(node.persistStore, finalized)
		if errors.Is(err, badger.ErrConflict) {
			time.Sleep(i)
			continue
		} else if err != nil {
			return err
		}

		err = node.persistStore.WriteTransaction(tx)
		if errors.Is(err, badger.ErrConflict) {
			time.Sleep(i)
			continue
		}
		return err
	}
	panic(fmt.Errorf("lockAndPersistTransaction timeout %v %v\n",
		tx.PayloadHash(), finalized))
}

func (node *Node) validateKernelSnapshot(s *common.Snapshot, tx *common.VersionedTransaction, finalized bool) error {
	if finalized && node.networkId.String() == config.KernelNetworkId &&
		s.Timestamp < mainnetConsensusReferenceForkAt {
		return nil
	}
	if s.NodeId != node.IdForNetwork && s.RoundNumber == 0 &&
		tx.TransactionType() != common.TransactionTypeNodeAccept {
		return fmt.Errorf("invalid initial transaction type %d", tx.TransactionType())
	}
	err := node.validateConsensusTransactionReferences(s, tx)
	if err != nil {
		return err
	}
	switch tx.TransactionType() {
	case common.TransactionTypeMint:
		if finalized && tx.Inputs[0].Mint.Batch < mainnetMintDayGapSkipForkBatch &&
			node.networkId.String() == config.KernelNetworkId {
			return nil
		}
		err := node.validateMintSnapshot(s, tx)
		if err != nil {
			logger.Printf("validateMintSnapshot ERROR %v %s %s\n",
				s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeNodePledge:
		err := node.validateNodePledgeSnapshot(s, tx, finalized)
		if err != nil {
			logger.Printf("validateNodePledgeSnapshot ERROR %v %s %s\n",
				s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeNodeCancel:
		err := node.validateNodeCancelSnapshot(s, tx, finalized)
		if err != nil {
			logger.Printf("validateNodeCancelSnapshot ERROR %v %s %s\n",
				s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeNodeAccept:
		err := node.validateNodeAcceptSnapshot(s, tx, finalized)
		if err != nil {
			logger.Printf("validateNodeAcceptSnapshot ERROR %v %s %s\n",
				s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeNodeRemove:
		err := node.validateNodeRemoveSnapshot(s, tx, finalized)
		if err != nil {
			logger.Printf("validateNodeRemoveSnapshot ERROR %v %s %s\n",
				s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeCustodianUpdateNodes:
		err := node.validateCustodianUpdateNodes(s, tx, finalized)
		if err != nil {
			logger.Printf("validateCustodianUpdateNodes ERROR %v %s %s\n",
				s, hex.EncodeToString(tx.PayloadMarshal()), err.Error())
			return err
		}
	case common.TransactionTypeCustodianSlashNodes:
		return fmt.Errorf("not implemented %v", tx)
	}
	return nil
}

func (node *Node) validateConsensusTransactionReferences(s *common.Snapshot, tx *common.VersionedTransaction) error {
	switch tx.TransactionType() {
	case common.TransactionTypeMint:
	case common.TransactionTypeNodePledge:
	case common.TransactionTypeNodeCancel:
	case common.TransactionTypeNodeAccept:
	case common.TransactionTypeNodeRemove:
	case common.TransactionTypeCustodianUpdateNodes:
	case common.TransactionTypeCustodianSlashNodes:
	default:
		return nil
	}

	if len(tx.References) < 1 {
		return fmt.Errorf("invalid consensus reference count %s", tx.PayloadHash())
	}
	last, _ := node.ReadLastConsensusSnapshotWithHack()
	if last.SoleTransaction() == tx.PayloadHash() {
		return nil
	}
	if tx.References[0] != last.SoleTransaction() {
		return fmt.Errorf("invalid consensus reference %s %s", tx.PayloadHash(), last.SoleTransaction())
	}
	if s.Timestamp <= last.Timestamp {
		return fmt.Errorf("invalid consensus timestamp %s %s", tx.PayloadHash(), last.SoleTransaction())
	}
	return nil
}
