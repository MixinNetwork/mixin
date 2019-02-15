package rpc

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/vmihailenco/msgpack"
)

func queueTransaction(store storage.Store, params []interface{}) (string, error) {
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

func getTransaction(store storage.Store, params []interface{}) (*common.SignedTransaction, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	hash, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}
	return store.ReadTransaction(hash)
}

func listSnapshots(store storage.Store, params []interface{}) ([]*common.SnapshotWithTopologicalOrder, error) {
	if len(params) != 3 {
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
	sig, err := strconv.ParseBool(fmt.Sprint(params[2]))
	if err != nil {
		return nil, err
	}

	snapshots, err := store.ReadSnapshotsSinceTopology(offset, count)
	if err != nil || sig {
		return snapshots, err
	}

	for i, _ := range snapshots {
		snapshots[i].Transaction.Signatures = nil
		snapshots[i].Signatures = nil
	}
	return snapshots, nil
}
