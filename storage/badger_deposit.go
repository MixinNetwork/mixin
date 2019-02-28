package storage

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
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
	if bytes.Compare(ival, tx[:]) == 0 {
		return nil
	}
	return fmt.Errorf("invalid lock %s %s", hex.EncodeToString(ival), hex.EncodeToString(tx[:]))
}

func (s *BadgerStore) LockDepositInput(deposit *common.DepositData, tx crypto.Hash, fork bool) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		key := graphDepositKey(deposit)
		ival, err := readDepositInput(txn, deposit)

		if err == badger.ErrKeyNotFound {
			return txn.Set(key, tx[:])
		}
		if err != nil {
			return err
		}

		if bytes.Compare(ival, tx[:]) != 0 {
			if !fork {
				return fmt.Errorf("deposit locked for transaction %s", hex.EncodeToString(ival))
			}
			var hash crypto.Hash
			copy(hash[:], ival)
			err := pruneTransaction(txn, hash)
			if err != nil {
				return err
			}
		}
		return txn.Set(key, tx[:])
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

func graphDepositKey(deposit *common.DepositData) []byte {
	hash := crypto.NewHash(common.MsgpackMarshalPanic(deposit))
	return append([]byte(graphPrefixDeposit), hash[:]...)
}
