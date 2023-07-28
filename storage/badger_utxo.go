package storage

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
	"golang.org/x/exp/slices"
)

func (s *BadgerStore) ReadUTXOKeys(hash crypto.Hash, index int) (*common.UTXOKeys, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	utxo, err := s.readUTXOLock(txn, hash, index)
	if err != nil {
		return nil, err
	}
	return &common.UTXOKeys{
		Mask: utxo.Mask,
		Keys: utxo.Keys,
	}, nil
}

func (s *BadgerStore) ReadUTXOLock(hash crypto.Hash, index int) (*common.UTXOWithLock, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return s.readUTXOLock(txn, hash, index)
}

func (s *BadgerStore) readUTXOLock(txn *badger.Txn, hash crypto.Hash, index int) (*common.UTXOWithLock, error) {
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

	return common.DecompressUnmarshalUTXO(ival)
}

func (s *BadgerStore) LockUTXOs(inputs []*common.Input, tx crypto.Hash, fork bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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

	out, err := common.DecompressUnmarshalUTXO(ival)
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
	return txn.Set(key, out.CompressMarshal())
}

func (s *BadgerStore) ReadGhostKeyLock(key crypto.Key) (*crypto.Hash, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	item, err := txn.Get(graphGhostKey(key))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var by crypto.Hash
	_, err = item.ValueCopy(by[:])
	return &by, err
}

func (s *BadgerStore) LockGhostKeys(keys []*crypto.Key, tx crypto.Hash, fork bool) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		for _, ghost := range keys {
			err := lockGhostKey(txn, ghost, tx, fork)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func lockGhostKey(txn *badger.Txn, ghost *crypto.Key, tx crypto.Hash, fork bool) error {
	key := graphGhostKey(*ghost)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return txn.Set(key, tx[:])
	}
	if err != nil {
		return err
	}
	var by crypto.Hash
	val, err := item.ValueCopy(by[:])
	if err != nil {
		return err
	}
	if len(val) != len(by) || !by.HasValue() {
		return fmt.Errorf("ghost key %s malformed lock %x", ghost.String(), val)
	}
	if fork && slices.Contains([]string{
		"c63b6373652def5999c1d951fcb8f064db67b7d18565847b921b21639e15dddd",
		"60deaf2471bb0b6481efe9080d8852b020ab2941e7faae21989d2404f34284ee",
		"a558b1efbe27eb6a6f902fd97d4b7e2e3099e6edde1fe6e8e41204e0685fe426",
	}, tx.String()) {
		return nil
	}
	if by != tx {
		return fmt.Errorf("ghost key %s locked for transaction %s", ghost.String(), by.String())
	}
	return nil
}
