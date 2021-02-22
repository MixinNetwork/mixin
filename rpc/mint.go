package rpc

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
)

func listMintWorks(node *kernel.Node, params []interface{}) (map[string]interface{}, error) {
	if len(params) != 1 {
		return nil, errors.New("invalid params count")
	}
	offset, err := strconv.ParseUint(fmt.Sprint(params[0]), 10, 64)
	if err != nil {
		return nil, err
	}

	works, err := node.ListMintWorks(offset)
	if err != nil {
		return nil, err
	}
	wm := make(map[string]interface{})
	for id, w := range works {
		wm[id.String()] = w
	}
	return wm, nil
}

func listMintDistributions(store storage.Store, params []interface{}) ([]map[string]interface{}, error) {
	if len(params) != 3 {
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
	tx, err := strconv.ParseBool(fmt.Sprint(params[2]))
	if err != nil {
		return nil, err
	}

	mints, transactions, err := store.ReadMintDistributions(common.MintGroupKernelNode, offset, count)
	return mintsToMap(mints, transactions, tx), err
}

func mintsToMap(mints []*common.MintDistribution, transactions []*common.VersionedTransaction, tx bool) []map[string]interface{} {
	tx = tx && len(transactions) == len(mints)
	result := make([]map[string]interface{}, len(mints))
	for i, m := range mints {
		item := map[string]interface{}{
			"group":  m.Group,
			"batch":  m.Batch,
			"amount": m.Amount,
		}
		if tx {
			item["transaction"] = transactionToMap(transactions[i])
		} else {
			item["transaction"] = m.Transaction
		}
		result[i] = item
	}
	return result
}
