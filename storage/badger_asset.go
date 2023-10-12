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
	default: // TODO more assets and better default value
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

	typ := ver.TransactionType()
	switch { // TODO needs full test code for all kind of transactions
	case typ == common.TransactionTypeWithdrawalSubmit:
		total = total.Sub(ver.Outputs[0].Amount)
	case typ == common.TransactionTypeDeposit:
		total = total.Add(ver.DepositData().Amount)
	case typ == common.TransactionTypeMint:
		total = total.Add(ver.Inputs[0].Mint.Amount)
	case len(ver.Inputs[0].Genesis) > 0:
		for _, out := range ver.Outputs {
			total = total.Add(out.Amount)
		}
	default:
		return nil
	}

	max := assetCapAt(ver.Asset)
	if total.Cmp(max) > 0 {
		panic(total.String())
	}
	key := graphAssetTotalKey(ver.Asset)
	return txn.Set(key, []byte(total.String()))
}

func graphAssetTotalKey(id crypto.Hash) []byte {
	return append([]byte(graphPrefixAssetTotal), id[:]...)
}
