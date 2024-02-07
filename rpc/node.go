package rpc

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/p2p"
	"github.com/MixinNetwork/mixin/storage"
)

func listAllNodes(store storage.Store, node *kernel.Node, params []any) ([]map[string]any, error) {
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
	result := make([]map[string]any, len(nodes))
	for i, n := range nodes {
		item := map[string]any{
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

func peerNeighbors(peers []*p2p.Peer) []map[string]any {
	sort.Slice(peers, func(i, j int) bool { return peers[i].IdForNetwork.String() < peers[j].IdForNetwork.String() })
	data := make([]map[string]any, 0)
	for _, p := range peers {
		data = append(data, map[string]any{
			"id":      p.IdForNetwork.String(),
			"address": p.Address,
		})
	}
	return data
}
