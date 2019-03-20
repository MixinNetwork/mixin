package storage

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
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
	err = common.MsgpackUnmarshal(ival, &out)
	return &out, err
}

func (s *BadgerStore) LockUTXO(hash crypto.Hash, index int, tx crypto.Hash, fork bool) (*common.UTXO, error) {
	var utxo *common.UTXO
	err := s.snapshotsDB.Update(func(txn *badger.Txn) error {
		key := graphUtxoKey(hash, index)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		ival, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		var out common.UTXOWithLock
		err = common.MsgpackUnmarshal(ival, &out)
		if err != nil {
			return err
		}

		if out.LockHash.HasValue() && out.LockHash != tx {
			if !fork {
				return fmt.Errorf("utxo locked for transaction %s", out.LockHash)
			}
			err := pruneTransaction(txn, out.LockHash)
			if err != nil {
				return err
			}
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
