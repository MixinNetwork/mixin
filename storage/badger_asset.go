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
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return common.Zero, err
	}
	return common.NewIntegerFromString(string(val)), nil
}

func writeTotalInAsset(txn *badger.Txn, ver *common.VersionedTransaction) error {
	amount := common.Zero
	switch ver.TransactionType() {
	case common.TransactionTypeWithdrawalSubmit:
		amount = amount.Sub(ver.Outputs[0].Amount)
	case common.TransactionTypeDeposit:
		amount = amount.Add(ver.Outputs[0].Amount)
	default:
		return nil
	}
	sum, err := readTotalInAsset(txn, ver.Asset)
	if err != nil {
		return err
	}
	sum = sum.Add(amount)
	key := graphAssetTotalKey(ver.Asset)
	return txn.Set(key, []byte(sum.String()))
}

func graphAssetTotalKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixAssetTotal), hash[:]...)
}
