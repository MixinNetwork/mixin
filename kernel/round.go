package kernel

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
)

type Round struct {
	NodeId crypto.Hash `msgpack:"N"`
	Number uint64      `msgpack:"R"`
	Start  uint64      `msgpack:"T"`
}

type RoundWithHash struct {
	Round
	Hash crypto.Hash
}

type RoundGraph struct {
	Nodes      []crypto.Hash
	CacheRound map[crypto.Hash]*Round
	FinalRound map[crypto.Hash]*RoundWithHash
	BestFinal  *RoundWithHash
}

func (g *RoundGraph) Print() string {
	desc := "ROUND GRAPH\n"
	for _, id := range g.Nodes {
		desc = desc + fmt.Sprintf("NODE# %s\n", id)
		final := g.FinalRound[id]
		desc = desc + fmt.Sprintf("FINAL %d %d %s\n", final.Number, final.Start, final.Hash)
		cache := g.CacheRound[id]
		if cache == nil {
			desc = desc + "CACHE NIL\n"
		} else {
			desc = desc + fmt.Sprintf("CACHE %d %d\n", cache.Number, cache.Start)
		}
	}
	desc = desc + fmt.Sprintf("BEST# %s", g.BestFinal.NodeId)
	return desc
}

func loadRoundGraph(store storage.Store) (*RoundGraph, error) {
	graph := &RoundGraph{
		CacheRound: make(map[crypto.Hash]*Round),
		FinalRound: make(map[crypto.Hash]*RoundWithHash),
	}
	nodes, err := store.SnapshotsNodeList()
	if err != nil {
		return nil, err
	}

	for _, id := range nodes {
		graph.Nodes = append(graph.Nodes, id)

		cache, err := loadHeadRoundForNode(store, id)
		if err != nil {
			return nil, err
		}
		graph.CacheRound[cache.NodeId] = cache

		finalRoundNumber := cache.Number - 1
		if cache.Number == 0 {
			finalRoundNumber = cache.Number
			delete(graph.CacheRound, cache.NodeId)
		}
		final, err := loadFinalRoundForNode(store, id, finalRoundNumber)
		if err != nil {
			return nil, err
		}
		graph.FinalRound[final.NodeId] = final
		graph.BestFinal = final
	}

	for _, r := range graph.FinalRound {
		if r.Start > graph.BestFinal.Start {
			graph.BestFinal = r
			break
		}
	}

	logger.Println("\n" + graph.Print())
	return graph, nil
}

func loadHeadRoundForNode(store storage.Store, nodeIdWithNetwork crypto.Hash) (*Round, error) {
	meta, err := store.SnapshotsRoundMetaForNode(nodeIdWithNetwork)
	if err != nil {
		return nil, err
	}

	round := &Round{
		NodeId: nodeIdWithNetwork,
		Number: meta[0],
		Start:  meta[1],
	}
	return round, nil
}

func loadFinalRoundForNode(store storage.Store, nodeIdWithNetwork crypto.Hash, number uint64) (*RoundWithHash, error) {
	snapshots, err := store.SnapshotsListForNodeRound(nodeIdWithNetwork, number)
	if err != nil {
		return nil, err
	}

	start := ^uint64(0)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, number)
	hashes := append(nodeIdWithNetwork[:], buf...)
	for _, s := range snapshots {
		h := crypto.NewHash(s.Payload())
		hashes = append(hashes, h[:]...)
		if s.Timestamp < start {
			start = s.Timestamp
		}
	}
	round := &RoundWithHash{
		Round: Round{
			NodeId: nodeIdWithNetwork,
			Number: number,
			Start:  start,
		},
		Hash: crypto.NewHash(hashes),
	}
	return round, nil
}
