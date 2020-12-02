package rpc

import (
	"fmt"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
)

func getInfo(store storage.Store, node *kernel.Node) (map[string]interface{}, error) {
	info := map[string]interface{}{
		"network":   node.NetworkId(),
		"node":      node.IdForNetwork,
		"version":   config.BuildVersion,
		"uptime":    node.Uptime().String(),
		"epoch":     time.Unix(0, int64(node.Epoch)),
		"timestamp": time.Unix(0, int64(node.GraphTimestamp)),
	}
	pool, err := node.PoolSize()
	if err != nil {
		return info, err
	}
	md, err := store.ReadLastMintDistribution(common.MintGroupKernelNode)
	if err != nil {
		return info, err
	}
	info["mint"] = map[string]interface{}{
		"pool":  pool,
		"batch": md.Batch,
	}
	cacheMap, finalMap, err := kernel.LoadRoundGraph(store, node.NetworkId(), node.IdForNetwork)
	if err != nil {
		return info, err
	}
	cacheGraph := make(map[string]interface{})
	for n, r := range cacheMap {
		for i := range r.Snapshots {
			r.Snapshots[i].Signatures = nil
		}
		cacheGraph[n.String()] = map[string]interface{}{
			"node":       r.NodeId.String(),
			"round":      r.Number,
			"timestamp":  r.Timestamp,
			"snapshots":  r.Snapshots,
			"references": r.References,
		}
	}
	finalGraph := make(map[string]interface{})
	for n, r := range finalMap {
		finalGraph[n.String()] = map[string]interface{}{
			"node":  r.NodeId.String(),
			"round": r.Number,
			"start": r.Start,
			"end":   r.End,
			"hash":  r.Hash.String(),
		}
	}

	nodes := make([]map[string]interface{}, 0)
	for _, n := range node.NodesListWithoutState(uint64(time.Now().UnixNano()), false) {
		switch n.State {
		case common.NodeStateAccepted, common.NodeStatePledging:
			nodes = append(nodes, map[string]interface{}{
				"node":        n.IdForNetwork,
				"signer":      n.Signer.String(),
				"payee":       n.Payee.String(),
				"state":       n.State,
				"timestamp":   n.Timestamp,
				"transaction": n.Transaction.String(),
			})
		}
	}
	info["graph"] = map[string]interface{}{
		"consensus": nodes,
		"cache":     cacheGraph,
		"final":     finalGraph,
		"topology":  node.TopologicalOrder(),
		"sps":       node.SPS(),
	}
	caches, finals := node.PoolInfo()
	info["queue"] = map[string]interface{}{
		"finals": finals,
		"caches": caches,
	}
	return info, nil
}

func dumpGraphHead(node *kernel.Node, params []interface{}) (interface{}, error) {
	rounds := node.BuildGraph()
	sort.Slice(rounds, func(i, j int) bool { return fmt.Sprint(rounds[i].NodeId) < fmt.Sprint(rounds[j].NodeId) })
	return rounds, nil
}
