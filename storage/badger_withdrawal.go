package storage

import (
	"encoding/hex"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ReadWithdrawalClaim(hash crypto.Hash) (*common.VersionedTransaction, string, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := graphWithdrawalClaimKey(hash)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, "", nil
	} else if err != nil {
		return nil, "", err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, "", err
	}

	var claim crypto.Hash
	if len(val) != len(claim) {
		panic(hex.EncodeToString(val))
	}
	copy(claim[:], val)
	return readTransactionAndFinalization(txn, claim)
}

func writeWithdrawalClaim(txn *badger.Txn, hash, claim crypto.Hash) error {
	tx, snap, err := readTransactionAndFinalization(txn, hash)
	if err != nil {
		return err
	}
	if tx == nil || len(snap) != 64 {
		panic(claim.String())
	}
	key := graphWithdrawalClaimKey(hash)
	return txn.Set(key, claim[:])
}

func graphWithdrawalClaimKey(tx crypto.Hash) []byte {
	return append([]byte(graphPrefixWithdrawal), tx[:]...)
}
