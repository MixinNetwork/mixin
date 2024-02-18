package server

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

func readWithdrawal(store storage.Store, params []any) (map[string]any, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	hash, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}

	ver, snap, err := store.ReadWithdrawalClaim(hash)
	if err != nil || ver == nil {
		return nil, err
	}

	data := transactionToMap(ver)
	data["hex"] = hex.EncodeToString(ver.Marshal())
	if len(snap) > 0 {
		data["snapshot"] = snap
	}
	return data, nil
}
