package rpc

import (
	"fmt"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
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
		"pool":   pool,
		"batch":  md.Batch,
		"pledge": node.PledgeAmount(node.GraphTimestamp),
	}
	cacheMap, finalMap := node.LoadRoundGraph()
	cacheGraph := make(map[string]interface{})
	for n, r := range cacheMap {
		sm := make([]map[string]interface{}, len(r.Snapshots))
		for i, s := range r.Snapshots {
			sm[i] = map[string]interface{}{
				"version":     s.Version,
				"node":        s.NodeId,
				"references":  roundLinkToMap(s.References),
				"round":       s.RoundNumber,
				"timestamp":   s.Timestamp,
				"hash":        s.Hash,
				"transaction": s.Transaction,
				"signature":   s.Signature,
			}
		}
		cacheGraph[n.String()] = map[string]interface{}{
			"node":       r.NodeId.String(),
			"round":      r.Number,
			"timestamp":  r.Timestamp,
			"snapshots":  sm,
			"references": roundLinkToMap(r.References),
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

	cids := make([]crypto.Hash, 0)
	nodes := make([]map[string]interface{}, 0)
	list := node.NodesListWithoutState(node.GraphTimestamp, false)
	for _, n := range list {
		switch n.State {
		case common.NodeStateAccepted, common.NodeStatePledging:
			cids = append(cids, n.IdForNetwork)
		}
	}
	offsets, err := store.ListWorkOffsets(cids)
	if err != nil {
		return info, err
	}
	works, err := store.ListNodeWorks(cids, uint32(node.GraphTimestamp/uint64(time.Hour*24)))
	if err != nil {
		return info, err
	}
	for _, n := range list {
		switch n.State {
		case common.NodeStateAccepted, common.NodeStatePledging:
			nodes = append(nodes, map[string]interface{}{
				"node":        n.IdForNetwork,
				"signer":      n.Signer.String(),
				"payee":       n.Payee.String(),
				"state":       n.State,
				"timestamp":   n.Timestamp,
				"transaction": n.Transaction.String(),
				"aggregator":  offsets[n.IdForNetwork],
				"works":       works[n.IdForNetwork],
			})
		}
	}
	info["graph"] = map[string]interface{}{
		"consensus": nodes,
		"cache":     cacheGraph,
		"final":     finalGraph,
		"topology":  node.TopologicalOrder(),
		"sps":       node.SPS(),
		"tps":       node.TPS(),
	}
	caches, finals, state := node.QueueState()
	info["queue"] = map[string]interface{}{
		"finals": finals,
		"caches": caches,
		"state":  state,
	}
	return info, nil
}

func dumpGraphHead(node *kernel.Node, params []interface{}) (interface{}, error) {
	rounds := node.BuildGraph()
	sort.Slice(rounds, func(i, j int) bool { return fmt.Sprint(rounds[i].NodeId) < fmt.Sprint(rounds[j].NodeId) })
	return rounds, nil
}
