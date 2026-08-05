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
	var missing []crypto.Hash
	var pending []*common.VersionedTransaction
	found := make(map[crypto.Hash]*common.VersionedTransaction)
	for _, txh := range s.Transactions {
		tx, snap, err := node.persistStore.ReadTransaction(txh)
		if err != nil {
			return nil, nil, err
		}
		if !finalized && len(snap) > 0 && snap != s.Hash.String() {
			return nil, nil, fmt.Errorf("transaction %s finalized in snapshot %s", txh, snap)
		}
		if tx != nil {
			found[txh] = tx
			err := node.validateKernelSnapshot(s, found, finalized)
			if err != nil {
				return nil, nil, err
			}
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

		err = tx.Validate(&ghostLockCollector{node.persistStore}, s.Timestamp, finalized)
		if err != nil {
			return nil, nil, err
		}

		found[txh] = tx
		err = node.validateKernelSnapshot(s, found, finalized)
		if err != nil {
			return nil, nil, err
		}
		pending = append(pending, tx)
	}
	if len(pending) > 0 {
		err := node.lockAndPersistTransactions(pending, s.Timestamp, finalized)
		if err != nil {
			return nil, nil, err
		}
	}

	return found, missing, nil
}

// ghostLockCollector skips the per-transaction ghost key commit during
// validation. The same keys are locked by LockAndPersistTransactions in the
// single batch commit, where any conflicting ownership still fails the
// snapshot, so skipping the individual commits here changes no outcome.
type ghostLockCollector struct {
	common.DataStore
}

func (c *ghostLockCollector) LockGhostKeys(keys []*crypto.Key, tx crypto.Hash, fork bool) error {
	return nil
}

// lockAndPersistTransactions locks and persists a whole snapshot batch in a
// single store commit. Transactions in one snapshot never depend on each
// other: a dependent transaction would fail validation against the store
// until its inputs are finalized, so validating first and persisting the
// batch afterwards keeps the same semantics with one fsync per snapshot
// instead of two per transaction.
func (node *Node) lockAndPersistTransactions(txs []*common.VersionedTransaction, snapTime uint64, finalized bool) error {
	err := node.lockAndPersistTransactionBatch(txs, finalized)
	if !errors.Is(err, badger.ErrTxnTooBig) {
		return err
	}

	// A finalized snapshot can also replace many existing input locks, adding
	// enough prune writes for the reduced batch to remain too large. Fall back
	// to bounded commits, matching the persistence behavior before batching.
	for _, tx := range txs {
		err := tx.Validate(node.persistStore, snapTime, finalized)
		if err != nil {
			return err
		}
		err = node.lockAndPersistTransactionBatch([]*common.VersionedTransaction{tx}, finalized)
		if err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) lockAndPersistTransactionBatch(txs []*common.VersionedTransaction, finalized bool) error {
	for i := time.Duration(0); i < time.Second; i += time.Millisecond * 100 {
		err := node.persistStore.LockAndPersistTransactions(txs, finalized)
		if errors.Is(err, badger.ErrConflict) {
			time.Sleep(i)
			continue
		}
		return err
	}
	panic(fmt.Errorf("lockAndPersistTransactionBatch timeout %d %v", len(txs), finalized))
}

func (node *Node) validateKernelSnapshot(s *common.Snapshot, found map[crypto.Hash]*common.VersionedTransaction, finalized bool) error {
	if len(s.Transactions) > 1 {
		for _, tx := range found {
			if !tx.IsSnapshotBatchable() {
				return fmt.Errorf("transaction %d is non batchable", tx.TransactionType())
			}
		}
		return nil
	}
	if finalized && node.networkId.String() == config.KernelNetworkId &&
		s.Timestamp < mainnetConsensusReferenceForkAt {
		return nil
	}
	tx := found[s.Transactions[0]]
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
	if len(s.Transactions) > 1 {
		panic(s.Hash)
	}
	switch tx.TransactionType() {
	case common.TransactionTypeMint:
	case common.TransactionTypeNodePledge:
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
