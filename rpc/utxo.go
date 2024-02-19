package rpc

import (
	"encoding/json"

	"github.com/MixinNetwork/mixin/common"
)

func GetUTXO(rpc, hash string, index uint64) (*common.UTXOWithLock, error) {
	data, err := callMixinRPC(rpc, "getutxo", []any{hash, index})
	if err != nil {
		return nil, err
	}
	var out common.UTXOWithLock
	err = json.Unmarshal(data, &out)
	if err != nil {
		panic(string(data))
	}
	return &out, err
}
