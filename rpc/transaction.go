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

func getCacheTransaction(store storage.Store, params []any) (map[string]any, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	hash, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}
	tx, err := store.CacheGetTransaction(hash)
	if err != nil || tx == nil {
		return nil, err
	}
	data := transactionToMap(tx)
	data["hex"] = hex.EncodeToString(tx.Marshal())
	return data, nil
}

func queueTransaction(node *kernel.Node, params []any) (string, error) {
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

func getTransaction(store storage.Store, params []any) (map[string]any, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	hash, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}
	tx, snap, err := store.ReadTransaction(hash)
	if err != nil || tx == nil {
		return nil, err
	}
	data := transactionToMap(tx)
	data["hex"] = hex.EncodeToString(tx.Marshal())
	if len(snap) > 0 {
		data["snapshot"] = snap
	}
	return data, nil
}

func getUTXO(store storage.Store, params []any) (map[string]any, error) {
	if len(params) != 2 {
		return nil, errors.New("invalid params count")
	}
	hash, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}
	index, err := strconv.ParseUint(fmt.Sprint(params[1]), 10, 64)
	if err != nil {
		return nil, err
	}
	utxo, err := store.ReadUTXOLock(hash, int(index))
	if err != nil || utxo == nil {
		return nil, err
	}

	output := map[string]any{
		"type":   utxo.Type,
		"hash":   hash,
		"index":  index,
		"amount": utxo.Amount,
	}
	if len(utxo.Keys) > 0 {
		output["keys"] = utxo.Keys
	}
	if len(utxo.Script) > 0 {
		output["script"] = utxo.Script
	}
	if utxo.Mask.HasValue() {
		output["mask"] = utxo.Mask
	}
	if utxo.LockHash.HasValue() {
		output["lock"] = utxo.LockHash
	}
	return output, nil
}

func getGhostKey(store storage.Store, params []any) (map[string]any, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	key, err := crypto.KeyFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}
	by, err := store.ReadGhostKeyLock(key)
	if err != nil {
		return nil, err
	}
	res := map[string]any{"transaction": nil}
	if by != nil {
		res["transaction"] = by.String()
	}
	return res, nil
}

func getSnapshot(node *kernel.Node, store storage.Store, params []any) (map[string]any, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	hash, err := crypto.HashFromString(fmt.Sprint(params[0]))
	if err != nil {
		return nil, err
	}
	snap, err := store.ReadSnapshot(hash)
	if err != nil || snap == nil {
		return nil, err
	}
	tx, _, err := store.ReadTransaction(snap.SoleTransaction())
	if err != nil {
		return nil, err
	}
	return snapshotToMap(node, snap, tx, true), nil
}

func listSnapshots(node *kernel.Node, store storage.Store, params []any) ([]map[string]any, error) {
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
		return snapshotsToMap(node, snapshots, transactions, sig), err
	}
	snapshots, err := store.ReadSnapshotsSinceTopology(offset, count)
	return snapshotsToMap(node, snapshots, nil, sig), err
}

func snapshotsToMap(node *kernel.Node, snapshots []*common.SnapshotWithTopologicalOrder, transactions []*common.VersionedTransaction, sig bool) []map[string]any {
	tx := len(transactions) == len(snapshots)
	result := make([]map[string]any, len(snapshots))
	for i, s := range snapshots {
		if tx {
			result[i] = snapshotToMap(node, s, transactions[i], sig)
		} else {
			result[i] = snapshotToMap(node, s, nil, sig)
		}
	}
	return result
}

func snapshotToMap(node *kernel.Node, s *common.SnapshotWithTopologicalOrder, tx *common.VersionedTransaction, sig bool) map[string]any {
	wn := node.WitnessSnapshot(s)
	item := map[string]any{
		"version":    s.Version,
		"node":       s.NodeId,
		"references": roundLinkToMap(s.References),
		"round":      s.RoundNumber,
		"timestamp":  s.Timestamp,
		"hash":       s.Hash,
		"hex":        hex.EncodeToString(s.VersionedMarshal()),
		"topology":   s.TopologicalOrder,
		"witness": map[string]any{
			"signature": wn.Signature,
			"timestamp": wn.Timestamp,
		},
	}
	if tx != nil {
		item["transactions"] = []any{transactionToMap(tx)}
	} else {
		item["transactions"] = []any{s.SoleTransaction()}
	}
	if sig {
		item["signature"] = s.Signature
	}
	return item
}

func transactionToMap(tx *common.VersionedTransaction) map[string]any {
	var inputs []map[string]any
	for _, in := range tx.Inputs {
		if in.Hash.HasValue() {
			inputs = append(inputs, map[string]any{
				"hash":  in.Hash,
				"index": in.Index,
			})
		} else if len(in.Genesis) > 0 {
			inputs = append(inputs, map[string]any{
				"genesis": hex.EncodeToString(in.Genesis),
			})
		} else if d := in.Deposit; d != nil {
			inputs = append(inputs, map[string]any{
				"deposit": map[string]any{
					"chain":            d.Chain,
					"asset_key":        d.AssetKey,
					"transaction_hash": d.TransactionHash,
					"output_index":     d.OutputIndex,
					"amount":           d.Amount,
				},
			})
		} else if m := in.Mint; m != nil {
			inputs = append(inputs, map[string]any{
				"mint": map[string]any{
					"group":  m.Group,
					"batch":  m.Batch,
					"amount": m.Amount,
				},
			})
		}
	}

	var outputs []map[string]any
	for _, out := range tx.Outputs {
		output := map[string]any{
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
		if w := out.Withdrawal; w != nil {
			output["withdrawal"] = map[string]any{
				"chain":     w.Chain,
				"asset_key": w.AssetKey,
				"address":   w.Address,
				"tag":       w.Tag,
			}
		}
		outputs = append(outputs, output)
	}

	tm := map[string]any{
		"version":    tx.Version,
		"asset":      tx.Asset,
		"inputs":     inputs,
		"outputs":    outputs,
		"extra":      hex.EncodeToString(tx.Extra),
		"hash":       tx.PayloadHash(),
		"references": tx.References,
	}
	return tm
}
