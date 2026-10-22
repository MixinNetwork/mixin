package rpc

import "github.com/MixinNetwork/mixin/common"

func GetDepositTransaction(rpc, chain, hash string, index uint64) (*common.VersionedTransaction, string, error) {
	raw, err := CallMixinRPC(rpc, "getdeposittransaction", []any{chain, hash, index})
	if err != nil || raw == nil {
		return nil, "", err
	}
	return decodeTransactionResponse(raw)
}
