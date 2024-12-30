package kernel

import (
	"fmt"
	"sort"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
)

type CacheRound struct {
	NodeId     crypto.Hash
	Number     uint64
	Timestamp  uint64
	References *common.RoundLink
	Snapshots  []*common.Snapshot
	index      map[crypto.Hash]bool
}

type FinalRound struct {
	NodeId crypto.Hash
	Number uint64
	Start  uint64
	End    uint64
	Hash   crypto.Hash
}

func (node *Node) LoadAllChainsAndGraphTimestamp(store storage.Store, networkId crypto.Hash) error {
	nodes := node.NodesListWithoutState(clock.NowUnixNano(), false)
	for _, cn := range nodes {
		chain := node.getOrCreateChain(cn.IdForNetwork)
		if chain.State == nil {
			continue
		}
		if t := chain.State.FinalRound.End; t > node.GraphTimestamp {
			node.GraphTimestamp = t
		}
	}
	logger.Printf("node.LoadAllChainsAndGraphTimestamp(%s) => %d %d", networkId, len(nodes), node.GraphTimestamp)

	node.chains.RLock()
	for _, chain := range node.chains.m {
		chain.bootLoops()
	}
	node.chains.RUnlock()
	return nil
}

func (node *Node) LoadRoundGraph() (map[crypto.Hash]*CacheRound, map[crypto.Hash]*FinalRound) {
	cacheRound := make(map[crypto.Hash]*CacheRound)
	finalRound := make(map[crypto.Hash]*FinalRound)

	node.chains.RLock()
	defer node.chains.RUnlock()

	for _, chain := range node.chains.m {
		if chain.State == nil {
			continue
		}
		c, f := chain.StateCopy()
		finalRound[chain.ChainId] = f
		cacheRound[chain.ChainId] = c
	}

	return cacheRound, finalRound
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
		index:      make(map[crypto.Hash]bool),
	}
	topos, err := store.ReadSnapshotsForNodeRound(round.NodeId, round.Number)
	if err != nil {
		return nil, err
	}
	for _, t := range topos {
		s := t.Snapshot
		s.Hash = s.PayloadHash()
		round.Snapshots = append(round.Snapshots, s)
		round.index[s.Hash] = true
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
		s := t.Snapshot
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
		index:     c.index,
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
	err := cache.validateSnapshot(s, true)
	if err != nil {
		panic(err)
	}
	chain.assignNewGraphRound(final, cache)
	chain.State.CacheRound.index[s.Hash] = true
	return nil
}

func (c *CacheRound) ValidateSnapshot(s *common.Snapshot) error {
	return c.validateSnapshot(s, false)
}

func (c *CacheRound) validateSnapshot(s *common.Snapshot, add bool) error {
	if s.RoundNumber != c.Number || !s.Hash.HasValue() {
		panic(s)
	}
	for _, cs := range c.Snapshots {
		if cs.Hash == s.Hash || cs.Timestamp == s.Timestamp || cs.SoleTransaction() == s.SoleTransaction() {
			return fmt.Errorf("ValidateSnapshot error duplication %s %d %s", s.Hash, s.Timestamp, s.SoleTransaction())
		}
		if cs.Timestamp/OneDay != s.Timestamp/OneDay {
			return fmt.Errorf("ValidateSnapshot error round day leap %s %d %s", s.Hash, s.Timestamp, s.SoleTransaction())
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

func (c *CacheRound) asFinal() *FinalRound {
	if len(c.Snapshots) == 0 {
		return nil
	}

	start, end, hash := common.ComputeRoundHash(c.NodeId, c.Number, c.Snapshots)
	round := &FinalRound{
		NodeId: c.NodeId,
		Number: c.Number,
		Start:  start,
		End:    end,
		Hash:   hash,
	}
	return round
}
