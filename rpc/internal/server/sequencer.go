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
	snapshots := make([]map[string]any, len(block.Snapshots))
	for i, hash := range block.Block.Snapshots {
		s := block.Snapshots[hash]
		tx := block.Transactions[s.SoleTransaction()]
		snapshots[i] = snapshotToMap(s, tx, true)
	}
	return map[string]any{
		"number":    block.Number,
		"sequence":  block.Sequence,
		"hash":      block.PayloadHash(),
		"timestamp": block.Timestamp,
		"previous":  block.Previous,
		"signature": block.Signature,
		"node":      block.NodeId,
		"snapshots": snapshots,
	}, nil
}
