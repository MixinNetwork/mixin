package storage

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ReadDepositLock(deposit *common.DepositData) (crypto.Hash, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	var hash crypto.Hash
	ival, err := readDepositInput(txn, deposit)
	if err == badger.ErrKeyNotFound {
		return hash, nil
	} else if err != nil {
		return hash, err
	}
	if len(ival) != len(hash) {
		panic(hex.EncodeToString(ival))
	}
	copy(hash[:], ival)
	return hash, nil
}

func (s *BadgerStore) LockDepositInput(deposit *common.DepositData, tx crypto.Hash, fork bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		ival, err := readDepositInput(txn, deposit)
		if err == badger.ErrKeyNotFound {
			return writeDepositLock(txn, deposit, tx)
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
		return writeDepositLock(txn, deposit, tx)
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

func writeDepositLock(txn *badger.Txn, deposit *common.DepositData, tx crypto.Hash) error {
	key := graphDepositKey(deposit)
	return txn.Set(key, tx[:])
}

func graphDepositKey(deposit *common.DepositData) []byte {
	hash := deposit.UniqueKey()
	return append([]byte(graphPrefixDeposit), hash[:]...)
}
