package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/bitcoin"
	"github.com/MixinNetwork/mixin/domains/ethereum"
	"github.com/dgraph-io/badger/v4"
)

func assetCapAt(id crypto.Hash) common.Integer {
	switch id {
	case bitcoin.BitcoinChainId:
		return common.NewIntegerFromString("10000")
	case ethereum.EthereumChainId:
		return common.NewIntegerFromString("100000")
	case common.XINAssetId:
		return common.NewIntegerFromString("1000000")
	default:
		return common.NewIntegerFromString("115792089237316195423570985008687907853269984665640564039457.58400791")
	}
}

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
		amount := ver.DepositData().Amount
		total = total.Add(amount)
		max := assetCapAt(ver.Asset)
		if amount.Cmp(max) > 0 || total.Cmp(max) > 0 {
			panic(amount.String())
		}
	default:
		return nil
	}

	key := graphAssetTotalKey(ver.Asset)
	return txn.Set(key, []byte(total.String()))
}

func graphAssetTotalKey(id crypto.Hash) []byte {
	return append([]byte(graphPrefixAssetTotal), id[:]...)
}
