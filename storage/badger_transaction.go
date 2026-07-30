package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ReadTransaction(hash crypto.Hash) (*common.VersionedTransaction, string, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readTransactionAndFinalization(txn, hash)
}

func readTransactionAndFinalization(txn *badger.Txn, hash crypto.Hash) (*common.VersionedTransaction, string, error) {
	tx, err := readTransaction(txn, hash)
	if err != nil || tx == nil {
		return tx, "", err
	}
	key := graphFinalizationKey(hash)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return tx, "", nil
	} else if err != nil {
		return tx, "", err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return tx, "", err
	}
	var final crypto.Hash
	if len(val) != len(final) {
		panic(len(val))
	}
	copy(final[:], val)
	return tx, final.String(), nil
}

func (s *BadgerStore) WriteTransaction(ver *common.VersionedTransaction) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	err := writeTransactionChecked(txn, newBatchClaims(), ver)
	if err != nil {
		return err
	}
	return txn.Commit()
}

// LockAndPersistTransactions locks the inputs and output ghost keys of every
// transaction and persists their bodies within a single badger transaction.
// Validating a snapshot batch previously required several fsynced commits
// per transaction, one for the ghost keys, one for the inputs and one for
// the body, which dominated the transaction-to-snapshot latency. One commit
// per batch preserves the same durability with a fraction of the fsyncs,
// and holding the store mutex for the whole batch keeps the serialization
// against WriteSnapshot unchanged.
//
// The new path writes every input lock, ghost lock, and transaction body in
// one Badger transaction. Badger’s default limit is roughly 104,857 entries,
// but one valid script transaction can contain 256 outputs × 256 keys =
// 65,536 ghost locks. Two such batchable transactions remain below the 4 MiB
// per-transaction and transport-size limits, yet require over 131,000 lock
// writes. This returns badger.ErrTxnTooBig on a verifier without pre-existing
// locks. Meanwhile, a proposer may succeed because queue validation already
// stored those locks individually, creating state-dependent snapshot acceptance.
// Cap expected writes before batching, or keep ghost-lock commits bounded/per-transaction.
func (s *BadgerStore) LockAndPersistTransactions(vers []*common.VersionedTransaction, fork bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		claims := newBatchClaims()
		for _, ver := range vers {
			err := lockTransactionInputs(txn, claims, ver, fork)
			if err != nil {
				return err
			}
		}
		for _, ver := range vers {
			err := lockTransactionGhosts(txn, claims, ver, fork)
			if err != nil {
				return err
			}
		}
		for _, ver := range vers {
			err := writeTransactionChecked(txn, claims, ver)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// batchClaims tracks lock ownership within a single batch commit. A badger
// transaction does not read its own pending writes, so the per-transaction
// lock conflict checks must be mirrored in memory to reject two transactions
// in the same snapshot claiming the same input or ghost key.
type batchClaims struct {
	utxos    map[string]crypto.Hash
	deposits map[crypto.Hash]crypto.Hash
	mints    map[uint64]crypto.Hash
	ghosts   map[crypto.Key]crypto.Hash
}

func newBatchClaims() *batchClaims {
	return &batchClaims{
		utxos:    make(map[string]crypto.Hash),
		deposits: make(map[crypto.Hash]crypto.Hash),
		mints:    make(map[uint64]crypto.Hash),
		ghosts:   make(map[crypto.Key]crypto.Hash),
	}
}

func lockTransactionInputs(txn *badger.Txn, claims *batchClaims, ver *common.VersionedTransaction, fork bool) error {
	tx := ver.PayloadHash()
	switch ver.TransactionType() {
	case common.TransactionTypeMint:
		mint := ver.Inputs[0].Mint
		if by, ok := claims.mints[mint.Batch]; ok && by != tx {
			return fmt.Errorf("mint locked for transaction %s", by.String())
		}
		if err := lockMintInput(txn, mint, tx, fork); err != nil {
			return err
		}
		claims.mints[mint.Batch] = tx
		return nil
	case common.TransactionTypeDeposit:
		deposit := ver.Inputs[0].Deposit
		key := deposit.UniqueKey()
		if by, ok := claims.deposits[key]; ok && by != tx {
			return fmt.Errorf("deposit locked for transaction %s", by.String())
		}
		if err := lockDepositInput(txn, deposit, tx, fork); err != nil {
			return err
		}
		claims.deposits[key] = tx
		return nil
	}
	for _, in := range ver.Inputs {
		fk := fmt.Sprintf("%s:%d", in.Hash, in.Index)
		if by, ok := claims.utxos[fk]; ok && by != tx {
			return fmt.Errorf("utxo locked for transaction %s", by.String())
		}
		err := lockUTXO(txn, in.Hash, in.Index, tx, fork)
		if err != nil {
			return err
		}
		claims.utxos[fk] = tx
	}
	return nil
}

func lockTransactionGhosts(txn *badger.Txn, claims *batchClaims, ver *common.VersionedTransaction, fork bool) error {
	tx := ver.PayloadHash()
	for _, o := range ver.Outputs {
		for _, k := range o.Keys {
			_, ok := claims.ghosts[*k]
			if ok {
				return fmt.Errorf("duplicated ghost key %s", k.String())
			}
			err := lockGhostKey(txn, k, tx, fork)
			if err != nil {
				return err
			}
			claims.ghosts[*k] = tx
		}
	}
	return nil
}

func writeTransactionChecked(txn *badger.Txn, claims *batchClaims, ver *common.VersionedTransaction) error {
	// FIXME assert kind checks, not needed at all
	if config.Debug {
		txHash := ver.PayloadHash()
		for _, in := range ver.Inputs {
			if len(in.Genesis) > 0 {
				continue
			}

			if in.Deposit != nil {
				if claims.deposits[in.Deposit.UniqueKey()] == txHash {
					continue
				}
				ival, err := readDepositInput(txn, in.Deposit)
				if err != nil {
					panic(fmt.Errorf("deposit check error %s", err.Error()))
				}
				if !bytes.Equal(ival, txHash[:]) {
					panic(fmt.Errorf("deposit locked for transaction %s", hex.EncodeToString(ival)))
				}
				continue
			}

			if in.Mint != nil {
				if claims.mints[in.Mint.Batch] == txHash {
					continue
				}
				dist, err := readMintInput(txn, in.Mint)
				if err != nil {
					panic(fmt.Errorf("mint check error %s", err.Error()))
				}
				if dist.Transaction != txHash || dist.Amount.Cmp(in.Mint.Amount) != 0 {
					panic(fmt.Errorf("mint locked for transaction %s", dist.Transaction.String()))
				}
				continue
			}

			if claims.utxos[fmt.Sprintf("%s:%d", in.Hash, in.Index)] == txHash {
				continue
			}
			key := graphUtxoKey(in.Hash, in.Index)
			item, err := txn.Get(key)
			if err != nil {
				panic(fmt.Errorf("UTXO check error %s %s:%d=>%s", err.Error(), in.Hash, in.Index, txHash))
			}
			ival, err := item.ValueCopy(nil)
			if err != nil {
				panic(fmt.Errorf("UTXO check error %s", err.Error()))
			}
			out, err := common.UnmarshalUTXO(ival)
			if err != nil {
				panic(fmt.Errorf("UTXO check error %s", err.Error()))
			}
			if out.LockHash != txHash {
				panic(fmt.Errorf("utxo %s:%d locked for transaction %s instead of %s", out.Hash, out.Index, out.LockHash, txHash))
			}
		}
	}
	// assert end

	return writeTransaction(txn, ver)
}

func readTransaction(txn *badger.Txn, hash crypto.Hash) (*common.VersionedTransaction, error) {
	key := graphTransactionKey(hash)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	return common.UnmarshalVersionedTransaction(val)
}

func pruneTransaction(txn *badger.Txn, hash crypto.Hash) error {
	key := graphFinalizationKey(hash)
	_, err := txn.Get(key)
	if err == nil {
		return fmt.Errorf("prune finalized transaction %s", hash)
	} else if err != badger.ErrKeyNotFound {
		return err
	}
	key = graphTransactionKey(hash)
	return txn.Delete(key)
}

func writeTransaction(txn *badger.Txn, ver *common.VersionedTransaction) error {
	key := graphTransactionKey(ver.PayloadHash())

	_, err := txn.Get(key)
	if err == nil {
		return nil
	} else if err != badger.ErrKeyNotFound {
		return err
	}

	if d := ver.Inputs[0].Deposit; d != nil {
		err := verifyAssetInfo(txn, ver.Asset, d.Asset())
		if err != nil {
			return err
		}
	}

	val := ver.Marshal()
	return txn.Set(key, val)
}

func finalizeTransaction(txn *badger.Txn, ver *common.VersionedTransaction, snap *common.SnapshotWithTopologicalOrder) error {
	key := graphFinalizationKey(ver.PayloadHash())
	_, err := txn.Get(key)
	if err == nil {
		return nil
	} else if err != badger.ErrKeyNotFound {
		return err
	}
	snapHash := snap.PayloadHash()
	err = txn.Set(key, snapHash[:])
	if err != nil {
		return err
	}

	if d := ver.Inputs[0].Deposit; d != nil {
		err := writeAssetInfo(txn, ver.Asset, d.Asset())
		if err != nil {
			return err
		}
	}

	genesis := len(ver.Inputs[0].Genesis) > 0
	for _, utxo := range ver.UnspentOutputs() {
		err := writeUTXO(txn, utxo, ver, snap.Timestamp, genesis)
		if err != nil {
			return err
		}
	}

	return writeTotalInAsset(txn, ver)
}

func writeUTXO(txn *badger.Txn, utxo *common.UTXOWithLock, ver *common.VersionedTransaction, timestamp uint64, genesis bool) error {
	for _, k := range utxo.Keys {
		err := lockGhostKey(txn, k, utxo.Hash, true)
		if err != nil {
			return err
		}
	}
	key := graphUtxoKey(utxo.Hash, utxo.Index)
	val := utxo.Marshal()
	err := txn.Set(key, val)
	if err != nil {
		return err
	}

	var signer, payee crypto.Key
	if len(ver.Extra) >= len(signer) {
		copy(signer[:], ver.Extra)
		copy(payee[:], ver.Extra[len(signer):])
	}
	switch utxo.Type {
	case common.OutputTypeNodePledge:
		return writeNodePledge(txn, signer, payee, utxo.Hash, timestamp)
	case common.OutputTypeNodeCancel:
		return writeNodeCancel(txn, signer, payee, utxo.Hash, timestamp)
	case common.OutputTypeNodeAccept:
		return writeNodeAccept(txn, signer, payee, utxo.Hash, timestamp, genesis)
	case common.OutputTypeNodeRemove:
		return writeNodeRemove(txn, signer, payee, utxo.Hash, timestamp)
	case common.OutputTypeCustodianUpdateNodes:
		return writeCustodianNodes(txn, timestamp, utxo, ver.Extra, genesis)
	case common.OutputTypeWithdrawalClaim:
		return writeWithdrawalClaim(txn, ver.References[0], ver.PayloadHash())
	}

	return nil
}

func graphTransactionKey(txh crypto.Hash) []byte {
	return append([]byte(graphPrefixTransaction), txh[:]...)
}

func graphFinalizationKey(txh crypto.Hash) []byte {
	return append([]byte(graphPrefixFinalization), txh[:]...)
}

func graphUniqueKey(nodeId, txh crypto.Hash) []byte {
	key := append(txh[:], nodeId[:]...)
	return append([]byte(graphPrefixUnique), key...)
}

func graphGhostKey(k crypto.Key) []byte {
	return append([]byte(graphPrefixGhost), k[:]...)
}

func graphUtxoKey(hash crypto.Hash, index uint) []byte {
	if index > 1024 {
		panic(index)
	}
	buf := make([]byte, binary.MaxVarintLen64)
	size := binary.PutVarint(buf, int64(index))
	key := append([]byte(graphPrefixUTXO), hash[:]...)
	return append(key, buf[:size]...)
}
