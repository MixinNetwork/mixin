package storage

import (
	"encoding/json"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) ReadAssetWithBalance(id crypto.Hash) (*common.Asset, common.Integer, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	asset, err := readAssetInfo(txn, id)
	if err != nil || asset == nil {
		return nil, common.Zero, err
	}
	balance, err := readTotalInAsset(txn, id)
	if err != nil {
		return nil, common.Zero, err
	}
	return asset, balance, nil
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
	asset, err := readAssetInfo(txn, ver.Asset)
	if err != nil {
		return err
	} else if asset == nil {
		panic(ver.Asset.String())
	}
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

	max := common.GetAssetCapacity(ver.Asset)
	if total.Cmp(max) > 0 {
		panic(total.String())
	}
	key := graphAssetTotalKey(ver.Asset)
	return txn.Set(key, []byte(total.String()))
}

func readAssetInfo(txn *badger.Txn, id crypto.Hash) (*common.Asset, error) {
	key := graphAssetInfoKey(id)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	var a common.Asset
	err = json.Unmarshal(val, &a)
	if err != nil {
		return nil, err
	}
	return &a, a.Verify()
}

func writeAssetInfo(txn *badger.Txn, id crypto.Hash, a *common.Asset) error {
	old, err := readAssetInfo(txn, id)
	if err != nil {
		return err
	}
	if old == nil {
		v, err := json.Marshal(a)
		if err != nil {
			panic(err)
		}
		key := graphAssetInfoKey(id)
		return txn.Set(key, v)
	}
	if old.Chain == a.Chain && old.AssetKey == a.AssetKey {
		return nil
	}
	return fmt.Errorf("invalid asset info %s %v %v", id.String(), *old, *a)
}

func graphAssetInfoKey(id crypto.Hash) []byte {
	return append([]byte(graphPrefixAssetInfo), id[:]...)
}

func graphAssetTotalKey(id crypto.Hash) []byte {
	return append([]byte(graphPrefixAssetTotal), id[:]...)
}
