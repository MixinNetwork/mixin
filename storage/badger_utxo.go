package storage

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v3"
)

func (s *BadgerStore) ReadUTXOKeys(hash crypto.Hash, index int) (*common.UTXOKeys, error) {
	utxo, err := s.ReadUTXOLock(hash, index)
	if err != nil {
		return nil, err
	}
	return &common.UTXOKeys{
		Mask: utxo.Mask,
		Keys: utxo.Keys,
	}, nil
}

func (s *BadgerStore) ReadUTXOLock(hash crypto.Hash, index int) (*common.UTXOWithLock, error) {
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

	var out common.UTXOWithLock
	err = common.DecompressMsgpackUnmarshal(ival, &out)
	return &out, err
}

func (s *BadgerStore) LockUTXOs(inputs []*common.Input, tx crypto.Hash, fork bool) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		for _, in := range inputs {
			err := lockUTXO(txn, in.Hash, in.Index, tx, fork)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func lockUTXO(txn *badger.Txn, hash crypto.Hash, index int, tx crypto.Hash, fork bool) error {
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
	err = common.DecompressMsgpackUnmarshal(ival, &out)
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
	return txn.Set(key, common.CompressMsgpackMarshalPanic(out))
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
