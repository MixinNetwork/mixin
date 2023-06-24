package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v3"
)

func readTotalInAsset(txn *badger.Txn, hash crypto.Hash) (common.Integer, error) {
	key := graphAssetTotalKey(hash)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return common.Zero, nil
	} else if err != nil {
		return common.Zero, err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return common.Zero, err
	}
	return common.NewIntegerFromString(string(val)), nil
}

func writeTotalInAsset(txn *badger.Txn, ver *common.VersionedTransaction) error {
	total, err := readTotalInAsset(txn, ver.Asset)
	if err != nil {
		return err
	}

	switch ver.TransactionType() {
	case common.TransactionTypeWithdrawalSubmit:
		total = total.Sub(ver.Outputs[0].Amount)
	case common.TransactionTypeDeposit:
		total = total.Add(ver.DepositData().Amount)
	default:
		return nil
	}

	key := graphAssetTotalKey(ver.Asset)
	return txn.Set(key, []byte(total.String()))
}

func graphAssetTotalKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixAssetTotal), hash[:]...)
}
