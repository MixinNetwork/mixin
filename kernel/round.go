package kernel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

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

		final, err := loadFinalRoundForNode(store, id, cache.Number-1)
		if err != nil {
			return nil, err
		}
		graph.FinalRound[final.NodeId] = final
		cache.Timestamp = final.Start + config.SnapshotRoundGap
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
	snapshots, err := store.ReadSnapshotsForNodeRound(round.NodeId, round.Number)
	if err != nil {
		return nil, err
	}
	for _, s := range snapshots {
		round.Snapshots = append(round.Snapshots, &s.Snapshot)
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

	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Timestamp < snapshots[j].Timestamp {
			return true
		}
		if snapshots[i].Timestamp > snapshots[j].Timestamp {
			return false
		}
		a, b := snapshots[i].PayloadHash(), snapshots[j].PayloadHash()
		return bytes.Compare(a[:], b[:]) < 0
	})
	start := snapshots[0].Timestamp
	end := snapshots[len(snapshots)-1].Timestamp

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, number)
	hash := crypto.NewHash(append(nodeIdWithNetwork[:], buf...))
	for _, s := range snapshots {
		ph := s.PayloadHash()
		hash = crypto.NewHash(append(hash[:], ph[:]...))
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
	start, end := (^uint64(0))/2, uint64(0)
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
	if !s.Hash.HasValue() {
		panic(s)
	}
	for _, cs := range c.Snapshots {
		if cs.Hash == s.Hash {
			return false
		}
	}
	if start, end := c.Gap(); start < end {
		if s.Timestamp < start && s.Timestamp+config.SnapshotRoundGap <= end {
			return false
		}
		if s.Timestamp > end && start+config.SnapshotRoundGap <= s.Timestamp {
			return false
		}
	}
	c.Snapshots = append(c.Snapshots, s)
	return true
}

func (c *CacheRound) asFinal() *FinalRound {
	if len(c.Snapshots) == 0 {
		return nil
	}

	sort.Slice(c.Snapshots, func(i, j int) bool {
		if c.Snapshots[i].Timestamp < c.Snapshots[j].Timestamp {
			return true
		}
		if c.Snapshots[i].Timestamp > c.Snapshots[j].Timestamp {
			return false
		}
		a, b := c.Snapshots[i].Hash, c.Snapshots[j].Hash
		return bytes.Compare(a[:], b[:]) < 0
	})
	start := c.Snapshots[0].Timestamp
	end := c.Snapshots[len(c.Snapshots)-1].Timestamp
	if end >= start+config.SnapshotRoundGap {
		panic(c.NodeId.String())
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, c.Number)
	hash := crypto.NewHash(append(c.NodeId[:], buf...))
	for _, s := range c.Snapshots {
		if s.Timestamp > end {
			panic(c.NodeId.String())
		}
		hash = crypto.NewHash(append(hash[:], s.Hash[:]...))
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
