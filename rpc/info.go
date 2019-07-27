package rpc

import (
	"time"

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
		"timestamp": time.Unix(0, int64(node.Graph.GraphTimestamp)),
	}
	graph, err := kernel.LoadRoundGraph(store, node.NetworkId(), node.IdForNetwork)
	if err != nil {
		return info, err
	}
	cacheGraph := make(map[string]interface{})
	for n, r := range graph.CacheRound {
		for i, _ := range r.Snapshots {
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
	for n, r := range graph.FinalRound {
		finalGraph[n.String()] = map[string]interface{}{
			"node":  r.NodeId.String(),
			"round": r.Number,
			"start": r.Start,
			"end":   r.End,
			"hash":  r.Hash.String(),
		}
	}

	nodes := make([]map[string]interface{}, 0)
	for id, n := range node.ConsensusNodes {
		nodes = append(nodes, map[string]interface{}{
			"node":      id,
			"signer":    n.Signer.String(),
			"payee":     n.Payee.String(),
			"state":     n.State,
			"timestamp": n.Timestamp,
		})
	}
	if n := node.ConsensusPledging; n != nil {
		nodes = append(nodes, map[string]interface{}{
			"node":      n.Signer.Hash().ForNetwork(node.NetworkId()),
			"signer":    n.Signer.String(),
			"payee":     n.Payee.String(),
			"state":     n.State,
			"timestamp": n.Timestamp,
		})
	}
	info["graph"] = map[string]interface{}{
		"consensus": nodes,
		"cache":     cacheGraph,
		"final":     finalGraph,
		"topology":  node.TopologicalOrder(),
	}
	f, c, err := store.QueueInfo()
	if err != nil {
		return info, err
	}
	info["queue"] = map[string]interface{}{
		"finals": f,
		"caches": c,
	}
	return info, nil
}
