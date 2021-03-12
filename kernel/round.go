package kernel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
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

func (node *Node) LoadAllChains(store storage.Store, networkId crypto.Hash) error {
	nodes := node.NodesListWithoutState(uint64(clock.Now().UnixNano()), false)
	for _, cn := range nodes {
		if cn.State == common.NodeStatePledging || cn.State == common.NodeStateCancelled {
			continue
		}

		chain := node.GetOrCreateChain(cn.IdForNetwork)
		if chain.State == nil {
			continue
		}
		if t := chain.State.FinalRound.End; t > node.GraphTimestamp {
			node.GraphTimestamp = t
		}
	}
	return nil
}

func LoadRoundGraph(store storage.Store, networkId, idForNetwork crypto.Hash) (map[crypto.Hash]*CacheRound, map[crypto.Hash]*FinalRound, error) {
	cacheRound := make(map[crypto.Hash]*CacheRound)
	finalRound := make(map[crypto.Hash]*FinalRound)

	allNodes := store.ReadAllNodes(uint64(clock.Now().UnixNano()), false)
	for _, cn := range allNodes {
		if cn.State == common.NodeStatePledging || cn.State == common.NodeStateCancelled {
			continue
		}

		id := cn.IdForNetwork(networkId)

		cache, err := loadHeadRoundForNode(store, id)
		if err != nil {
			return nil, nil, err
		}
		cacheRound[cache.NodeId] = cache

		final, err := loadFinalRoundForNode(store, id, cache.Number-1)
		if err != nil {
			return nil, nil, err
		}
		finalRound[final.NodeId] = final
		cache.Timestamp = final.Start + config.SnapshotRoundGap
	}

	return cacheRound, finalRound, nil
}

func loadRoundHistoryForNode(store storage.Store, to *FinalRound) []*FinalRound {
	var history []*FinalRound
	start := to.Number + 1 - config.SnapshotReferenceThreshold
	if to.Number+1 < config.SnapshotReferenceThreshold {
		start = 0
	}
	for ; start <= to.Number; start++ {
		r, err := loadFinalRoundForNode(store, to.NodeId, start)
		if err != nil {
			panic(err)
		}
		history = append(history, r)
	}
	return reduceHistory(history)
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

func (f *FinalRound) Common() *common.Round {
	return &common.Round{
		Hash:      f.Hash,
		NodeId:    f.NodeId,
		Number:    f.Number,
		Timestamp: f.Start,
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

func (chain *Chain) AddSnapshot(final *FinalRound, cache *CacheRound, s *common.Snapshot, signers []crypto.Hash) error {
	chain.node.TopoWrite(s, signers)
	if err := cache.validateSnapshot(s, true); err != nil {
		panic("should never be here")
	}
	chain.assignNewGraphRound(final, cache)
	return nil
}

func (c *CacheRound) ValidateSnapshot(s *common.Snapshot) error {
	return c.validateSnapshot(s, false)
}

func (c *CacheRound) validateSnapshot(s *common.Snapshot, add bool) error {
	if s.RoundNumber != c.Number || !s.Hash.HasValue() {
		panic(s)
	}
	day := uint64(time.Hour) * 24
	fork := uint64(SnapshotRoundDayLeapForkHack.UnixNano())
	for _, cs := range c.Snapshots {
		if cs.Hash == s.Hash || cs.Timestamp == s.Timestamp || cs.Transaction == s.Transaction {
			return fmt.Errorf("ValidateSnapshot error duplication %s %d %s", s.Hash, s.Timestamp, s.Transaction)
		}
		if cs.Timestamp >= fork && cs.Timestamp/day != s.Timestamp/day {
			return fmt.Errorf("ValidateSnapshot error round day leap %s %d %s", s.Hash, s.Timestamp, s.Transaction)
		}
	}
	if start, end := c.Gap(); start <= end {
		if s.Timestamp < start && s.Timestamp+config.SnapshotRoundGap <= end {
			return fmt.Errorf("ValidateSnapshot error gap start %s %d %d %d", s.Hash, s.Timestamp, start, end)
		}
		if s.Timestamp > end && start+config.SnapshotRoundGap <= s.Timestamp {
			return fmt.Errorf("ValidateSnapshot error gap end %s %d %d %d", s.Hash, s.Timestamp, start, end)
		}
	}
	if add {
		c.Snapshots = append(c.Snapshots, s)
	}
	return nil
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
