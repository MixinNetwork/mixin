package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

func (s *BadgerStore) ReadTransaction(hash crypto.Hash) (*common.Transaction, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	return readTransaction(txn, hash)
}

func (s *BadgerStore) WriteTransaction(tx *common.Transaction) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	// FIXME assert kind checks, not needed at all
	if config.Debug {
		txHash := tx.PayloadHash()
		for _, in := range tx.Inputs {
			if len(in.Genesis) > 0 {
				continue
			}

			if in.Deposit != nil {
				ival, err := readDepositInput(txn, in.Deposit)
				if err != nil {
					panic(fmt.Errorf("deposit check error %s", err.Error()))
				}
				if bytes.Compare(ival, txHash[:]) != 0 {
					panic(fmt.Errorf("deposit locked for transaction %s", hex.EncodeToString(ival)))
				}
				continue
			}

			key := graphUtxoKey(in.Hash, in.Index)
			item, err := txn.Get(key)
			if err != nil {
				panic(fmt.Errorf("UTXO check error %s", err.Error()))
			}
			ival, err := item.ValueCopy(nil)
			if err != nil {
				panic(fmt.Errorf("UTXO check error %s", err.Error()))
			}
			var out common.UTXOWithLock
			err = msgpack.Unmarshal(ival, &out)
			if err != nil {
				panic(fmt.Errorf("UTXO check error %s", err.Error()))
			}
			if out.LockHash != txHash {
				panic(fmt.Errorf("utxo locked for transaction %s", out.LockHash))
			}
		}
	}
	// assert end

	err := writeTransaction(txn, tx)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) CheckTransactionFinalization(hash crypto.Hash) (bool, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := graphFinalizationKey(hash)
	_, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *BadgerStore) CheckTransactionInNode(nodeId, hash crypto.Hash) (bool, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := graphUniqueKey(nodeId, hash)
	_, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func readTransaction(txn *badger.Txn, hash crypto.Hash) (*common.Transaction, error) {
	var out common.Transaction
	key := graphTransactionKey(hash)
	err := graphReadValue(txn, key, &out)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return &out, err
}

func writeTransaction(txn *badger.Txn, tx *common.Transaction) error {
	key := graphTransactionKey(tx.PayloadHash())

	// FIXME assert only, remove in future
	if config.Debug {
		_, err := txn.Get(key)
		if err == nil {
			panic("transaction duplication")
		} else if err != badger.ErrKeyNotFound {
			return err
		}
	}
	// end assert

	val := common.MsgpackMarshalPanic(tx)
	return txn.Set(key, val)
}

func finalizeTransaction(txn *badger.Txn, tx *common.Transaction) error {
	key := graphFinalizationKey(tx.PayloadHash())
	_, err := txn.Get(key)
	if err == nil {
		return nil
	} else if err != badger.ErrKeyNotFound {
		return err
	}
	err = txn.Set(key, []byte{})
	if err != nil {
		return err
	}

	var genesis bool
	for _, in := range tx.Inputs {
		if len(in.Genesis) > 0 {
			genesis = true
			break
		}
	}

	for _, utxo := range tx.UnspentOutputs() {
		err := writeUTXO(txn, utxo, tx.Extra, genesis)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeUTXO(txn *badger.Txn, utxo *common.UTXO, extra []byte, genesis bool) error {
	for _, k := range utxo.Keys {
		key := graphGhostKey(k)

		// FIXME assert kind checks, not needed at all
		if config.Debug {
			_, err := txn.Get(key)
			if err == nil {
				panic("ErrorValidateFailed")
			} else if err != badger.ErrKeyNotFound {
				return err
			}
		}
		// assert end

		err := txn.Set(key, []byte{0})
		if err != nil {
			return err
		}
	}
	key := graphUtxoKey(utxo.Hash, utxo.Index)
	val := common.MsgpackMarshalPanic(utxo)
	err := txn.Set(key, val)
	if err != nil {
		return err
	}

	switch utxo.Type {
	case common.OutputTypeNodePledge:
		var publicSpend crypto.Key
		copy(publicSpend[:], extra)
		return writeNodePledge(txn, publicSpend, utxo.Hash)
	case common.OutputTypeNodeAccept:
		var publicSpend crypto.Key
		copy(publicSpend[:], extra)
		return writeNodeAccept(txn, publicSpend, utxo.Hash, genesis)
	case common.OutputTypeDomainAccept:
		var publicSpend crypto.Key
		copy(publicSpend[:], extra)
		return writeDomainAccept(txn, publicSpend, utxo.Hash)
	}

	return nil
}

func graphTransactionKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixTransaction), hash[:]...)
}

func graphFinalizationKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixFinalization), hash[:]...)
}

func graphUniqueKey(nodeId, hash crypto.Hash) []byte {
	key := append(hash[:], nodeId[:]...)
	return append([]byte(graphPrefixUnique), key...)
}

func graphGhostKey(k crypto.Key) []byte {
	return append([]byte(graphPrefixGhost), k[:]...)
}

func graphUtxoKey(hash crypto.Hash, index int) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	size := binary.PutVarint(buf, int64(index))
	key := append([]byte(graphPrefixUTXO), hash[:]...)
	return append(key, buf[:size]...)
}

func graphDepositKey(deposit *common.DepositData) []byte {
	hash := crypto.NewHash(common.MsgpackMarshalPanic(deposit))
	return append([]byte(graphPrefixDeposit), hash[:]...)
}
