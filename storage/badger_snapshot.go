package storage

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

func (s *BadgerStore) ReadUTXO(hash crypto.Hash, index int) (*common.UTXO, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := graphUtxoKey(hash, index)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}

	var out common.UTXO
	err = msgpack.Unmarshal(ival, &out)
	return &out, err
}

func readDepositInput(txn *badger.Txn, deposit *common.DepositData) ([]byte, error) {
	key := graphDepositKey(deposit)
	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(nil)
}

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
		save := func() error {
			return txn.Set(key, tx[:])
		}
		if err == badger.ErrKeyNotFound {
			return save()
		}
		if err != nil {
			return err
		}
		if !fork && bytes.Compare(ival, tx[:]) != 0 {
			return fmt.Errorf("deposit locked for transaction %s", hex.EncodeToString(ival))
		}
		return save()
	})
}

func (s *BadgerStore) LockUTXO(hash crypto.Hash, index int, tx crypto.Hash, fork bool) (*common.UTXO, error) {
	var utxo *common.UTXO
	err := s.snapshotsDB.Update(func(txn *badger.Txn) error {
		key := graphUtxoKey(hash, index)
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		ival, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		var out common.UTXOWithLock
		err = msgpack.Unmarshal(ival, &out)
		if err != nil {
			return err
		}

		if !fork && out.LockHash.HasValue() && out.LockHash != tx {
			return fmt.Errorf("utxo locked for transaction %s", out.LockHash)
		}
		out.LockHash = tx
		err = txn.Set(key, common.MsgpackMarshalPanic(out))
		utxo = &out.UTXO
		return err
	})
	return utxo, err
}

func (s *BadgerStore) CheckGhost(key crypto.Key) (bool, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	_, err := txn.Get(graphGhostKey(key))
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
