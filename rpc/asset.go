package rpc

import (
	"errors"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

func readAsset(store storage.Store, params []any) (map[string]any, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	id, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}

	asset, balance, err := store.ReadAssetWithBalance(id)
	if err != nil || asset == nil {
		return nil, err
	}
	return map[string]any{
		"id":      id,
		"chain":   asset.Chain,
		"asset":   asset.AssetKey,
		"balance": balance,
	}, nil
}
