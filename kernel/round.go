package kernel

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
)

type CacheRound struct {
	NodeId     crypto.Hash
	Number     uint64
	Timestamp  uint64
	References [2]crypto.Hash
	Snapshots  []*common.Snapshot `msgpack:"-"`
}

type FinalRound struct {
	NodeId crypto.Hash
	Number uint64
	Start  uint64
	End    uint64
	Hash   crypto.Hash
}

type RoundGraph struct {
	Nodes      []crypto.Hash
	CacheRound map[crypto.Hash]*CacheRound
	FinalRound map[crypto.Hash]*FinalRound
	FinalCache []FinalRound
}

func (g *RoundGraph) UpdateFinalCache() {
	finals := make([]FinalRound, 0)
	for _, f := range g.FinalRound {
		finals = append(finals, FinalRound{
			NodeId: f.NodeId,
			Number: f.Number,
			Start:  f.Start,
		})
	}
	g.FinalCache = finals
}

func (g *RoundGraph) Print() string {
	desc := "ROUND GRAPH BEGIN\n"
	for _, id := range g.Nodes {
		desc = desc + fmt.Sprintf("NODE# %s\n", id)
		final := g.FinalRound[id]
		desc = desc + fmt.Sprintf("FINAL %d %d %s\n", final.Number, final.Start, final.Hash)
		cache := g.CacheRound[id]
		desc = desc + fmt.Sprintf("CACHE %d %d\n", cache.Number, cache.Timestamp)
	}
	desc = desc + "ROUND GRAPH END"
	return desc
}

func LoadRoundGraph(store storage.Store, networkId crypto.Hash) (*RoundGraph, error) {
	graph := &RoundGraph{
		CacheRound: make(map[crypto.Hash]*CacheRound),
		FinalRound: make(map[crypto.Hash]*FinalRound),
	}

	consensusNodes := store.ReadConsensusNodes()
	for _, cn := range consensusNodes {
		id := cn.Account.Hash().ForNetwork(networkId)
		graph.Nodes = append(graph.Nodes, id)

		cache, err := loadHeadRoundForNode(store, id)
		if err != nil {
			return nil, err
		}
		if cache == nil {
			continue
		}
		graph.CacheRound[cache.NodeId] = cache

		finalRoundNumber := cache.Number - 1
		if cache.Number == 0 {
			finalRoundNumber = cache.Number
			graph.CacheRound[id] = &CacheRound{
				NodeId: id,
				Number: 1,
			}
		}
		final, err := loadFinalRoundForNode(store, id, finalRoundNumber)
		if err != nil {
			return nil, err
		}
		graph.FinalRound[final.NodeId] = final
	}

	logger.Println("\n" + graph.Print())
	graph.UpdateFinalCache()
	return graph, nil
}

func loadHeadRoundForNode(store storage.Store, nodeIdWithNetwork crypto.Hash) (*CacheRound, error) {
	meta, err := store.ReadRound(nodeIdWithNetwork)
	if err != nil || meta == nil {
		return nil, err
	}

	round := &CacheRound{
		NodeId:     nodeIdWithNetwork,
		Number:     meta.Number,
		Timestamp:  meta.Timestamp,
		References: meta.References,
	}
	round.Snapshots, err = store.ReadSnapshotsForNodeRound(round.NodeId, round.Number)
	if err != nil {
		return nil, err
	}
	return round, nil
}

func loadFinalRoundForNode(store storage.Store, nodeIdWithNetwork crypto.Hash, number uint64) (*FinalRound, error) {
	snapshots, err := store.ReadSnapshotsForNodeRound(nodeIdWithNetwork, number)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		panic(nodeIdWithNetwork)
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, number)
	start, end := ^uint64(0), uint64(0)
	hash := crypto.NewHash(append(nodeIdWithNetwork[:], buf...))
	for _, s := range snapshots {
		hash = hash.ByteOr(s.PayloadHash())
		if s.Timestamp < start {
			start = s.Timestamp
		}
		if s.Timestamp > end {
			end = s.Timestamp
		}
	}
	round := &FinalRound{
		NodeId: nodeIdWithNetwork,
		Number: number,
		Start:  start,
		End:    end,
		Hash:   hash,
	}
	if round.End-round.Start >= config.SnapshotRoundGap {
		panic(round)
	}
	return round, nil
}

func (c *CacheRound) Copy() *CacheRound {
	r := *c
	r.Snapshots = append([]*common.Snapshot{}, c.Snapshots...)
	return &r
}

func (f *FinalRound) Copy() *FinalRound {
	r := *f
	return &r
}

func (c *CacheRound) Gap() (uint64, uint64) {
	start, end := ^uint64(0), uint64(0)
	for _, s := range c.Snapshots {
		if s.Timestamp < start {
			start = s.Timestamp
		}
		if s.Timestamp > end {
			end = s.Timestamp
		}
	}
	return start, end
}

func (c *CacheRound) AddSnapshot(s *common.Snapshot) bool {
	for _, cs := range c.Snapshots {
		if cs.PayloadHash() == s.PayloadHash() {
			return false
		}
	}
	c.Snapshots = append(c.Snapshots, s)
	return true
}

func (c *CacheRound) FilterByHash(store storage.Store, ref crypto.Hash) error {
	filter := make([]*common.Snapshot, 0)
	for _, cs := range c.Snapshots {
		ph := cs.PayloadHash()
		if ph.ByteAnd(ref) == ph {
			filter = append(filter, cs)
		} else if err := store.PruneSnapshot(nil); err != nil {
			// FIXME cs to topo
			return err
		}
	}
	c.Snapshots = filter
	return nil
}

func (c *CacheRound) asFinal() *FinalRound {
	if len(c.Snapshots) == 0 {
		panic(c)
	}

	start, end := ^uint64(0), uint64(0)
	for _, s := range c.Snapshots {
		if s.Timestamp < start {
			start = s.Timestamp
		}
		if s.Timestamp > end {
			end = s.Timestamp
		}
	}
	if end >= start+config.SnapshotRoundGap {
		end = start + config.SnapshotRoundGap - 1
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, c.Number)
	hash := crypto.NewHash(append(c.NodeId[:], buf...))
	for _, s := range c.Snapshots {
		if s.Timestamp > end {
			continue
		}
		hash = hash.ByteOr(s.PayloadHash())
	}
	round := &FinalRound{
		NodeId: c.NodeId,
		Number: c.Number,
		Start:  start,
		End:    end,
		Hash:   hash,
	}
	return round
}
