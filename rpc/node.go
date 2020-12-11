package rpc

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
)

func listAllNodes(store storage.Store, node *kernel.Node, params []interface{}) ([]map[string]interface{}, error) {
	if len(params) != 2 {
		return nil, errors.New("invalid params count")
	}
	threshold, err := strconv.ParseUint(fmt.Sprint(params[0]), 10, 64)
	if err != nil {
		return nil, err
	}
	state, err := strconv.ParseBool(fmt.Sprint(params[1]))
	if err != nil {
		return nil, err
	}
	if threshold == 0 {
		threshold = uint64(time.Now().UnixNano())
	}
	nodes := store.ReadAllNodes(threshold, state)
	result := make([]map[string]interface{}, len(nodes))
	for i, n := range nodes {
		item := map[string]interface{}{
			"id":          n.IdForNetwork(node.NetworkId()),
			"signer":      n.Signer,
			"payee":       n.Payee,
			"transaction": n.Transaction,
			"timestamp":   n.Timestamp,
			"state":       n.State,
		}
		result[i] = item
	}
	return result, nil
}
