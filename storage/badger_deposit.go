package storage

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v3"
)

func (s *BadgerStore) CheckDepositInput(deposit *common.DepositData, tx crypto.Hash) error {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	ival, err := readDepositInput(txn, deposit)
	if err == badger.ErrKeyNotFound {
		return nil
	} else if err != nil {
		return err
	}
	if bytes.Equal(ival, tx[:]) {
		return nil
	}
	return fmt.Errorf("invalid lock %s %s", hex.EncodeToString(ival), hex.EncodeToString(tx[:]))
}

func (s *BadgerStore) LockDepositInput(deposit *common.DepositData, tx crypto.Hash, fork bool) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		ival, err := readDepositInput(txn, deposit)
		if err == badger.ErrKeyNotFound {
			return writeDeposit(txn, deposit, tx)
		}
		if err != nil {
			return err
		}

		if bytes.Equal(ival, tx[:]) {
			return nil
		}

		if !fork {
			return fmt.Errorf("deposit locked for transaction %s", hex.EncodeToString(ival))
		}
		var hash crypto.Hash
		copy(hash[:], ival)
		err = pruneTransaction(txn, hash)
		if err != nil {
			return err
		}
		return writeDeposit(txn, deposit, tx)
	})
}

func readDepositInput(txn *badger.Txn, deposit *common.DepositData) ([]byte, error) {
	key := graphDepositKey(deposit)
	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(nil)
}

func writeDeposit(txn *badger.Txn, deposit *common.DepositData, tx crypto.Hash) error {
	key := graphDepositKey(deposit)
	return txn.Set(key, tx[:])
}

func graphDepositKey(deposit *common.DepositData) []byte {
	hash := deposit.UniqueKey()
	return append([]byte(graphPrefixDeposit), hash[:]...)
}
