package server

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

func getBlock(store storage.Store, params []any) (map[string]any, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	var block *common.BlockWithTransactions
	hash, _ := crypto.HashFromString(fmt.Sprint(params[0]))
	if hash.HasValue() {
		bws, err := store.ReadBlockByHash(hash)
		if err != nil {
			return nil, err
		}
		block = bws
	} else {
		number, err := strconv.ParseUint(fmt.Sprint(params[0]), 10, 64)
		if err != nil {
			return nil, err
		}
		bws, err := store.ReadBlockWithTransactions(number)
		if err != nil {
			return nil, err
		}
		block = bws
	}
	return map[string]any{
		"number":    block.Number,
		"sequence":  block.Sequence,
		"hash":      block.PayloadHash(),
		"timestamp": block.Timestamp,
	}, nil
}
