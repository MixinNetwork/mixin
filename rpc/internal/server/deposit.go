package server

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

func readDeposit(store storage.Store, params []any) (map[string]any, error) {
	if len(params) != 3 {
		return nil, errors.New("invalid params count")
	}
	chain, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}
	hash := fmt.Sprint(params[1])
	index, err := strconv.ParseUint(fmt.Sprint(params[2]), 10, 64)
	if err != nil {
		return nil, err
	}

	dd := &common.DepositData{
		Chain:       chain,
		Transaction: hash,
		Index:       index,
	}
	locked, err := store.ReadDepositLock(dd)
	if err != nil || !locked.HasValue() {
		return nil, err
	}
	ver, snap, err := store.ReadTransaction(locked)
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
