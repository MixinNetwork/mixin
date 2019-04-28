package kernel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/network"
	"github.com/MixinNetwork/mixin/storage"
)

type CacheRound struct {
	NodeId     crypto.Hash
	Number     uint64
	Timestamp  uint64
	References *common.RoundLink
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
	Nodes      []*crypto.Hash
	CacheRound map[crypto.Hash]*CacheRound
	FinalRound map[crypto.Hash]*FinalRound

	RoundHistory map[crypto.Hash][]*FinalRound

	GraphTimestamp uint64
	FinalCache     []*network.SyncPoint
	MyCacheRound   *FinalRound
	MyFinalNumber  uint64
}

func (g *RoundGraph) UpdateFinalCache(idForNetwork crypto.Hash) {
	finals := make([]*network.SyncPoint, 0)
	for _, f := range g.FinalRound {
		finals = append(finals, &network.SyncPoint{
			NodeId: f.NodeId,
			Number: f.Number,
			Hash:   f.Hash,
		})
		if f.End > g.GraphTimestamp {
			g.GraphTimestamp = f.End
		}
	}
	g.FinalCache = finals
	g.MyCacheRound = g.CacheRound[idForNetwork].asFinal()
	g.MyFinalNumber = g.FinalRound[idForNetwork].Number
}

func (g *RoundGraph) Print() string {
	desc := "ROUND GRAPH BEGIN\n"
	for _, id := range g.Nodes {
		desc = desc + fmt.Sprintf("NODE# %s\n", id)
		final := g.FinalRound[*id]
		desc = desc + fmt.Sprintf("FINAL %d %d %s\n", final.Number, final.Start, final.Hash)
		cache := g.CacheRound[*id]
		start, end := cache.Gap()
		desc = desc + fmt.Sprintf("CACHE %d %d %d %d\n", cache.Number, cache.Timestamp, start, end)
	}
	desc = desc + "ROUND GRAPH END"
	return desc
}

func LoadRoundGraph(store storage.Store, networkId, idForNetwork crypto.Hash) (*RoundGraph, error) {
	graph := &RoundGraph{
		CacheRound:   make(map[crypto.Hash]*CacheRound),
		FinalRound:   make(map[crypto.Hash]*FinalRound),
		RoundHistory: make(map[crypto.Hash][]*FinalRound),
	}

	consensusNodes := store.ReadConsensusNodes()
	for _, cn := range consensusNodes {
		id := cn.Signer.Hash().ForNetwork(networkId)
		graph.Nodes = append(graph.Nodes, &id)

		cache, err := loadHeadRoundForNode(store, id)
		if err != nil {
			return nil, err
		}
		graph.CacheRound[cache.NodeId] = cache

		final, err := loadFinalRoundForNode(store, id, cache.Number-1)
		if err != nil {
			return nil, err
		}
		graph.FinalRound[final.NodeId] = final
		graph.RoundHistory[final.NodeId] = []*FinalRound{final.Copy()}
		cache.Timestamp = final.Start + config.SnapshotRoundGap
	}

	graph.UpdateFinalCache(idForNetwork)
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
	topos, err := store.ReadSnapshotsForNodeRound(round.NodeId, round.Number)
	if err != nil {
		return nil, err
	}
	for _, t := range topos {
		s := &t.Snapshot
		s.Hash = s.PayloadHash()
		round.Snapshots = append(round.Snapshots, s)
	}
	return round, nil
}

func loadFinalRoundForNode(store storage.Store, nodeIdWithNetwork crypto.Hash, number uint64) (*FinalRound, error) {
	topos, err := store.ReadSnapshotsForNodeRound(nodeIdWithNetwork, number)
	if err != nil {
		return nil, err
	}
	if len(topos) == 0 {
		panic(nodeIdWithNetwork)
	}

	snapshots := make([]*common.Snapshot, len(topos))
	for i, t := range topos {
		s := &t.Snapshot
		s.Hash = s.PayloadHash()
		snapshots[i] = s
	}
	cache := &CacheRound{
		NodeId:    nodeIdWithNetwork,
		Number:    number,
		Snapshots: snapshots,
	}
	return cache.asFinal(), nil
}

func (c *CacheRound) Copy() *CacheRound {
	return &CacheRound{
		NodeId:    c.NodeId,
		Number:    c.Number,
		Timestamp: c.Timestamp,
		References: &common.RoundLink{
			Self:     c.References.Self,
			External: c.References.External,
		},
		Snapshots: append([]*common.Snapshot{}, c.Snapshots...),
	}
}

func (f *FinalRound) Copy() *FinalRound {
	return &FinalRound{
		NodeId: f.NodeId,
		Number: f.Number,
		Start:  f.Start,
		End:    f.End,
		Hash:   f.Hash,
	}
}

func (c *CacheRound) Gap() (uint64, uint64) {
	start, end := (^uint64(0))/2, uint64(0)
	count := len(c.Snapshots)
	if count == 0 {
		return start, end
	}
	sort.Slice(c.Snapshots, func(i, j int) bool {
		return c.Snapshots[i].Timestamp < c.Snapshots[j].Timestamp
	})
	start = c.Snapshots[0].Timestamp
	end = c.Snapshots[count-1].Timestamp
	if end >= start+config.SnapshotRoundGap {
		err := fmt.Errorf("GAP %s %d %d %d %d", c.NodeId, c.Number, start, end, start+config.SnapshotRoundGap)
		panic(err)
	}
	return start, end
}

func (c *CacheRound) ValidateSnapshot(s *common.Snapshot, add bool) bool {
	if !s.Hash.HasValue() {
		panic(s)
	}
	for _, cs := range c.Snapshots {
		if cs.Hash == s.Hash || cs.Timestamp == s.Timestamp {
			return false
		}
	}
	if start, end := c.Gap(); start <= end {
		if s.Timestamp < start && s.Timestamp+config.SnapshotRoundGap <= end {
			return false
		}
		if s.Timestamp > end && start+config.SnapshotRoundGap <= s.Timestamp {
			return false
		}
	}
	if add {
		c.Snapshots = append(c.Snapshots, s)
	}
	return true
}

func ComputeRoundHash(nodeId crypto.Hash, number uint64, snapshots []*common.Snapshot) (uint64, uint64, crypto.Hash) {
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Timestamp < snapshots[j].Timestamp {
			return true
		}
		if snapshots[i].Timestamp > snapshots[j].Timestamp {
			return false
		}
		a, b := snapshots[i].Hash, snapshots[j].Hash
		return bytes.Compare(a[:], b[:]) < 0
	})
	start := snapshots[0].Timestamp
	end := snapshots[len(snapshots)-1].Timestamp
	if end >= start+config.SnapshotRoundGap {
		err := fmt.Errorf("ComputeRoundHash(%s, %d) %d %d %d", nodeId, number, start, end, start+config.SnapshotRoundGap)
		panic(err)
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, number)
	hash := crypto.NewHash(append(nodeId[:], buf...))
	for _, s := range snapshots {
		if s.Timestamp > end {
			panic(nodeId)
		}
		hash = crypto.NewHash(append(hash[:], s.Hash[:]...))
	}
	return start, end, hash
}

func (c *CacheRound) asFinal() *FinalRound {
	if len(c.Snapshots) == 0 {
		return nil
	}

	start, end, hash := ComputeRoundHash(c.NodeId, c.Number, c.Snapshots)
	round := &FinalRound{
		NodeId: c.NodeId,
		Number: c.Number,
		Start:  start,
		End:    end,
		Hash:   hash,
	}
	return round
}
