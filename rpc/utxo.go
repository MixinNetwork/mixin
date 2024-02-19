package rpc

import (
	"encoding/json"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func GetUTXO(rpc, hash string, index uint64) (*common.UTXOWithLock, error) {
	data, err := callMixinRPC(rpc, "getutxo", []any{hash, index})
	if err != nil {
		return nil, err
	}
	var out struct {
		Type     uint8          `json:"type"`
		Hash     crypto.Hash    `json:"hash"`
		Index    uint           `json:"index"`
		Amount   common.Integer `json:"amount"`
		Keys     []*crypto.Key  `json:"keys"`
		Script   common.Script  `json:"script"`
		Mask     *crypto.Key    `json:"mask"`
		LockHash crypto.Hash    `json:"lock"`
	}
	err = json.Unmarshal(data, &out)
	if err != nil {
		panic(string(data))
	}

	utxo := &common.UTXOWithLock{LockHash: out.LockHash}
	utxo.Type = out.Type
	utxo.Hash = out.Hash
	utxo.Index = out.Index
	utxo.Amount = out.Amount
	utxo.Keys = out.Keys
	utxo.Script = out.Script
	utxo.Mask = *out.Mask
	return utxo, nil
}
