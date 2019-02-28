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

func listSnapshots(store storage.Store, params []interface{}) ([]map[string]interface{}, error) {
	if len(params) != 4 {
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
	tx, err := strconv.ParseBool(fmt.Sprint(params[3]))
	if err != nil {
		return nil, err
	}

	if tx {
		snapshots, transactions, err := store.ReadSnapshotWithTransactionsSinceTopology(offset, count)
		return snapshotsToMap(snapshots, transactions, sig), err
	}
	snapshots, err := store.ReadSnapshotsSinceTopology(offset, count)
	return snapshotsToMap(snapshots, nil, sig), err
}

func snapshotsToMap(snapshots []*common.SnapshotWithTopologicalOrder, transactions []*common.Transaction, sig bool) []map[string]interface{} {
	tx := len(transactions) == len(snapshots)
	result := make([]map[string]interface{}, len(snapshots))
	for i, s := range snapshots {
		item := map[string]interface{}{
			"node":       s.NodeId,
			"references": s.References,
			"round":      s.RoundNumber,
			"timestamp":  s.Timestamp,
			"hash":       s.Hash,
		}
		if tx {
			item["transaction"] = transactions[i]
		} else {
			item["transaction"] = s.Transaction
		}
		if sig {
			item["signatures"] = s.Signatures
		}
		result[i] = item
	}
	return result
}
