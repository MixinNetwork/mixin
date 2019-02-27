package rpc

import (
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
)

func getInfo(store storage.Store) (map[string]interface{}, error) {
	info := map[string]interface{}{
		"network": kernel.NetworkId(),
		"node":    kernel.NodeIdForNetwork(),
		"version": config.BuildVersion,
	}
	graph, err := kernel.LoadRoundGraph(store, kernel.NetworkId(), kernel.NodeIdForNetwork())
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
	info["graph"] = map[string]interface{}{
		"consensus": kernel.ConsensusNodes(),
		"cache":     cacheGraph,
		"final":     finalGraph,
		"topology":  kernel.TopologicalOrder(),
	}
	t, f, c, err := store.QueueInfo()
	if err != nil {
		return info, err
	}
	info["queue"] = map[string]interface{}{
		"transactions": t,
		"finals":       f,
		"caches":       c,
	}
	return info, nil
}
