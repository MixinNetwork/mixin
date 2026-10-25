package rpc

import (
	"encoding/json"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func GetUTXO(rpc, hash string, index uint64) (*common.UTXOWithLock, error) {
	data, err := CallMixinRPC(rpc, "getutxo", []any{hash, index})
	if err != nil || data == nil {
		return nil, err
	}
	var out struct {
		Type     uint8          `json:"type"`
		Hash     crypto.Hash    `json:"hash"`
		Index    uint           `json:"index"`
		Asset    crypto.Hash    `json:"asset"`
		Amount   common.Integer `json:"amount"`
		Keys     []*crypto.Key  `json:"keys"`
		Script   common.Script  `json:"script"`
		Mask     *crypto.Key    `json:"mask"`
		LockHash crypto.Hash    `json:"lock"`
	}
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, fmt.Errorf("decode UTXO response: %w", err)
	}

	utxo := &common.UTXOWithLock{LockHash: out.LockHash}
	utxo.Type = out.Type
	utxo.Hash = out.Hash
	utxo.Index = out.Index
	utxo.Asset = out.Asset
	utxo.Amount = out.Amount
	utxo.Keys = out.Keys
	utxo.Script = out.Script
	if out.Mask != nil {
		utxo.Mask = *out.Mask
	}
	return utxo, nil
}
