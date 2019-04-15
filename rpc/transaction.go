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
)

func queueTransaction(node *kernel.Node, params []interface{}) (string, error) {
	if len(params) != 1 {
		return "", errors.New("invalid params count")
	}
	raw, err := hex.DecodeString(fmt.Sprint(params[0]))
	if err != nil {
		return "", err
	}
	ver, err := common.UnmarshalVersionedTransaction(raw)
	if err != nil {
		return "", err
	}
	return node.QueueTransaction(ver)
}

func getTransaction(store storage.Store, params []interface{}) (map[string]interface{}, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	hash, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}
	tx, err := store.ReadTransaction(hash)
	if err != nil || tx == nil {
		return nil, err
	}
	return transactionToMap(tx), nil
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

func snapshotsToMap(snapshots []*common.SnapshotWithTopologicalOrder, transactions []*common.VersionedTransaction, sig bool) []map[string]interface{} {
	tx := len(transactions) == len(snapshots)
	result := make([]map[string]interface{}, len(snapshots))
	for i, s := range snapshots {
		item := map[string]interface{}{
			"node":       s.NodeId,
			"references": s.References,
			"round":      s.RoundNumber,
			"timestamp":  s.Timestamp,
			"hash":       s.Hash,
			"topology":   s.TopologicalOrder,
		}
		if tx {
			item["transaction"] = transactionToMap(transactions[i])
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

func transactionToMap(tx *common.VersionedTransaction) map[string]interface{} {
	var inputs []map[string]interface{}
	for _, in := range tx.Inputs {
		if in.Hash.HasValue() {
			inputs = append(inputs, map[string]interface{}{
				"hash":  in.Hash,
				"index": in.Index,
			})
		} else if len(in.Genesis) > 0 {
			inputs = append(inputs, map[string]interface{}{
				"genesis": hex.EncodeToString(in.Genesis),
			})
		} else if in.Deposit != nil {
			inputs = append(inputs, map[string]interface{}{
				"deposit": in.Deposit,
			})
		} else if in.Mint != nil {
			inputs = append(inputs, map[string]interface{}{
				"mint": in.Mint,
			})
		}
	}

	var outputs []map[string]interface{}
	for _, out := range tx.Outputs {
		output := map[string]interface{}{
			"type":   out.Type,
			"amount": out.Amount,
		}
		if len(out.Keys) > 0 {
			output["keys"] = out.Keys
		}
		if len(out.Script) > 0 {
			output["script"] = out.Script
		}
		if out.Mask.HasValue() {
			output["mask"] = out.Mask
		}
		if out.Withdrawal != nil {
			output["withdrawal"] = out.Withdrawal
		}
		outputs = append(outputs, output)
	}

	return map[string]interface{}{
		"version": tx.Version,
		"asset":   tx.Asset,
		"inputs":  inputs,
		"outputs": outputs,
		"extra":   hex.EncodeToString(tx.Extra),
		"hash":    tx.PayloadHash(),
	}
}
