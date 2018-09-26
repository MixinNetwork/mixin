package rpc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/vmihailenco/msgpack"
)

func createTransaction(store storage.Store, params []interface{}) (string, error) {
	if len(params) != 1 {
		return "", errors.New("invalid params count")
	}
	raw, err := hex.DecodeString(fmt.Sprint(params[0]))
	if err != nil {
		return "", err
	}
	var tx common.SignedTransaction
	err = msgpack.Unmarshal(raw, &tx)
	if err != nil {
		return "", err
	}
	return kernel.QueueTransaction(store, &tx)
}

func listSnapshots(store storage.Store, params []interface{}) ([]*common.SnapshotWithTopologicalOrder, error) {
	if len(params) != 2 {
		return nil, errors.New("invalid params count")
	}
	offset, err := strconv.ParseUint(fmt.Sprint(params[0]), 10, 64)
	if err != nil {
		return nil, err
	}
	count, err := strconv.ParseUint(fmt.Sprint(params[1]), 10, 64)
	if err != nil {
		return nil, err
	}

	return store.SnapshotsListSince(offset, count)
}
