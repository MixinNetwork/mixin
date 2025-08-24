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

func (node *Node) validateSnapshotTransaction(s *common.Snapshot, finalized bool) (map[crypto.Hash]*common.VersionedTransaction, []crypto.Hash, error) {
	if s.RoundNumber == 0 && len(s.Transactions) != 1 {
		return nil, nil, fmt.Errorf("invalid first snapshot %s %d", s.NodeId, len(s.Transactions))
	}

	var (
		missing []crypto.Hash
		found   = make(map[crypto.Hash]*common.VersionedTransaction)
	)

	for _, txh := range s.Transactions {
		tx, _, err := node.persistStore.ReadTransaction(txh)
		if err != nil {
			return nil, nil, err
		}
		if tx != nil {
			found[txh] = tx
			continue
		}

		tx, err = node.persistStore.CacheGetTransaction(txh)
		if err != nil {
			return nil, nil, err
		}
		if tx == nil {
			missing = append(missing, txh)
			continue
		}

		err = tx.Validate(node.persistStore, s.Timestamp, finalized)
		if err != nil {
			return nil, nil, err
		}

		err = node.lockAndPersistTransaction(tx, finalized)
		if err != nil {
			return nil, nil, err
		}
		found[txh] = tx
	}

	if len(found) == len(s.Transactions) {
		err := node.validateKernelSnapshot(s, found, finalized)
		if err != nil {
			return nil, nil, err
		}
	}
	return found, missing, nil
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
	panic(fmt.Errorf("lockAndPersistTransaction timeout %v %v", tx.PayloadHash(), finalized))
}

func (node *Node) validateKernelSnapshot(s *common.Snapshot, found map[crypto.Hash]*common.VersionedTransaction, finalized bool) error {
	if len(found) != len(s.Transactions) {
		panic(len(found))
	}
	for _, tx := range found {
		if !tx.IsSnapshotBatchable() && len(s.Transactions) > 1 {
			return fmt.Errorf("transaction %d is non batchable", tx.TransactionType())
		}
	}
	if finalized && node.networkId.String() == config.KernelNetworkId &&
		s.Timestamp < mainnetConsensusReferenceForkAt {
		return nil
	}
	if len(s.Transactions) > 1 {
		return nil
	}
	tx := found[s.Transactions[0]]
	if s.NodeId != node.IdForNetwork && s.RoundNumber == 0 &&
		tx.TransactionType() != common.TransactionTypeNodeAccept {
		return fmt.Errorf("invalid initial transaction type %d", tx.TransactionType())
	}
	err := node.validateConsensusTransactionReferences(s, found)
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

func (node *Node) validateConsensusTransactionReferences(s *common.Snapshot, found map[crypto.Hash]*common.VersionedTransaction) error {
	if len(s.Transactions) > 1 {
		return nil
	}
	tx := found[s.Transactions[0]]
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
	if len(last.Transactions) > 1 {
		return fmt.Errorf("invalid consensus snapshot with multiple transactions %s", last.PayloadHash())
	}
	ltx := last.Transactions[0]
	if ltx == tx.PayloadHash() {
		return nil
	}
	if tx.References[0] != ltx {
		return fmt.Errorf("invalid consensus reference %s %s", tx.PayloadHash(), ltx)
	}
	if s.Timestamp <= last.Timestamp {
		return fmt.Errorf("invalid consensus timestamp %s %s", tx.PayloadHash(), ltx)
	}
	return nil
}
