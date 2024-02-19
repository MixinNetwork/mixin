package rpc

import (
	"encoding/hex"
	"encoding/json"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func GetTransaction(rpc, hash string) (*common.VersionedTransaction, string, error) {
	raw, err := callMixinRPC(rpc, "gettransaction", []any{hash})
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
	if signed["snapshot"] == nil {
		return ver, "", nil
	}
	return ver, signed["snapshot"].(string), nil
}

func SendRawTransaction(rpc, raw string) (crypto.Hash, error) {
	body, err := callMixinRPC(rpc, "sendrawtransaction", []any{raw})
	if err != nil {
		return crypto.Hash{}, err
	}
	var tx map[string]string
	err = json.Unmarshal(body, &tx)
	if err != nil {
		panic(string(body))
	}
	hash, err := crypto.HashFromString(tx["hash"])
	if err != nil || !hash.HasValue() {
		panic(string(body))
	}
	return hash, nil
}
