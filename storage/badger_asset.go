package storage

import (
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

func (s *BadgerStore) WriteAsset(a *common.Asset) error {
	if !a.AssetId().HasValue() {
		return fmt.Errorf("invalid asset %s %s", a.ChainId.String(), a.AssetKey)
	}

	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	key := graphAssetKey(a.AssetId())
	val := common.MsgpackMarshalPanic(a)
	err := txn.Set(key, val)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) ReadAsset(id crypto.Hash) (*common.Asset, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	ival, err := txn.Get(graphAssetKey(id))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v, err := ival.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	var a common.Asset
	err = common.MsgpackUnmarshal(v, &a)
	return &a, err
}

func graphAssetKey(id crypto.Hash) []byte {
	return append([]byte(graphPrefixAsset), id[:]...)
}
