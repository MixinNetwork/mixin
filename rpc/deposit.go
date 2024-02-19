package rpc

import (
	"encoding/hex"
	"encoding/json"

	"github.com/MixinNetwork/mixin/common"
)

func GetDepositTransaction(rpc, chain, hash string, index uint64) (*common.VersionedTransaction, string, error) {
	raw, err := callMixinRPC(rpc, "getdeposittransaction", []any{chain, hash, index})
	if err != nil || raw == nil {
		return nil, "", err
	}
	var signed map[string]any
	err = json.Unmarshal(raw, &signed)
	if err != nil {
		panic(string(raw))
	}
	hex, err := hex.DecodeString(signed["hex"].(string))
	if err != nil {
		panic(string(raw))
	}
	ver, err := common.UnmarshalVersionedTransaction(hex)
	if err != nil {
		panic(string(raw))
	}
	return ver, signed["snapshot"].(string), nil
}
