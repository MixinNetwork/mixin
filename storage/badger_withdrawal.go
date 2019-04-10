package storage

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

func (s *BadgerStore) LockWithdrawalClaim(hash crypto.Hash, tx crypto.Hash, fork bool) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		key := graphWithdrawalClaimKey(hash)
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return txn.Set(key, tx[:])
		}
		if err != nil {
			return err
		}

		ival, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if bytes.Compare(ival, tx[:]) == 0 {
			return nil
		}

		if !fork && bytes.Compare(ival, tx[:]) < 0 {
			return fmt.Errorf("withdrawal claim locked for transaction %s", hex.EncodeToString(ival))
		}
		var old crypto.Hash
		copy(old[:], ival)
		err = pruneTransaction(txn, old)
		if err != nil {
			return err
		}
		return txn.Set(key, tx[:])
	})
}

func graphWithdrawalClaimKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixWithdrawalClaim), hash[:]...)
}
