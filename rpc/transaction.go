package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func GetSnapshot(rpc, hash string) (*common.SnapshotWithTopologicalOrder, error) {
	raw, err := CallMixinRPC(rpc, "getsnapshot", []any{hash})
	if err != nil || raw == nil {
		return nil, err
	}
	var signed struct {
		Hex string `json:"hex"`
	}
	err = json.Unmarshal(raw, &signed)
	if err != nil {
		return nil, fmt.Errorf("decode snapshot response: %w", err)
	}
	payload, err := hex.DecodeString(signed.Hex)
	if err != nil {
		return nil, fmt.Errorf("decode snapshot hex: %w", err)
	}
	ver, err := common.UnmarshalVersionedSnapshot(payload)
	if err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	return ver, nil
}

func GetTransaction(rpc, hash string) (*common.VersionedTransaction, string, error) {
	raw, err := CallMixinRPC(rpc, "gettransaction", []any{hash})
	if err != nil || raw == nil {
		return nil, "", err
	}
	return decodeTransactionResponse(raw)
}

func decodeTransactionResponse(raw []byte) (*common.VersionedTransaction, string, error) {
	var signed struct {
		Hex      string `json:"hex"`
		Snapshot string `json:"snapshot"`
	}
	err := json.Unmarshal(raw, &signed)
	if err != nil {
		return nil, "", fmt.Errorf("decode transaction response: %w", err)
	}
	payload, err := hex.DecodeString(signed.Hex)
	if err != nil {
		return nil, "", fmt.Errorf("decode transaction hex: %w", err)
	}
	ver, err := common.UnmarshalVersionedTransaction(payload)
	if err != nil {
		return nil, "", fmt.Errorf("decode transaction: %w", err)
	}
	return ver, signed.Snapshot, nil
}

func SendRawTransaction(rpc, raw string) (crypto.Hash, error) {
	body, err := CallMixinRPC(rpc, "sendrawtransaction", []any{raw})
	if err != nil {
		return crypto.Hash{}, err
	}
	var tx struct {
		Hash string `json:"hash"`
	}
	err = json.Unmarshal(body, &tx)
	if err != nil {
		return crypto.Hash{}, fmt.Errorf("decode submission response: %w", err)
	}
	hash, err := crypto.HashFromString(tx.Hash)
	if err != nil {
		return crypto.Hash{}, fmt.Errorf("decode submission hash: %w", err)
	}
	if !hash.HasValue() {
		return crypto.Hash{}, fmt.Errorf("invalid submission hash")
	}
	return hash, nil
}
