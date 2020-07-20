package kernel

import (
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

const (
	FinalPoolSlotsLimit     = config.SnapshotSyncRoundThreshold * 8
	CachePoolSnapshotsLimit = 1024
)

type ChainRound struct {
	NodeId     crypto.Hash
	Number     uint64
	Timestamp  uint64
	References *common.RoundLink
	Snapshots  []*CosiAction
	finalSet   map[crypto.Hash]bool
}

type ChainCache struct {
	NodeId    crypto.Hash
	Number    uint64
	Snapshots *RingBuffer
}

type ChainState struct {
	sync.RWMutex
	CacheRound        *CacheRound
	FinalRound        *FinalRound
	RoundHistory      []*FinalRound
	ReverseRoundLinks map[crypto.Hash]uint64
}

type Chain struct {
	node    *Node
	ChainId crypto.Hash

	State *ChainState

	CosiAggregators map[crypto.Hash]*CosiAggregator
	CosiVerifiers   map[crypto.Hash]*CosiVerifier
	CachePool       *ChainCache
	FinalPool       [FinalPoolSlotsLimit]*ChainRound
	FinalIndex      int

	persistStore    storage.Store
	cosiActionsChan chan *CosiAction

	clc chan struct{}
}

func (node *Node) BuildChain(chainId crypto.Hash) *Chain {
	chain := &Chain{
		node:            node,
		ChainId:         chainId,
		State:           &ChainState{ReverseRoundLinks: make(map[crypto.Hash]uint64)},
		CosiAggregators: make(map[crypto.Hash]*CosiAggregator),
		CosiVerifiers:   make(map[crypto.Hash]*CosiVerifier),
		CachePool:       &ChainCache{NodeId: chainId, Snapshots: NewRingBuffer(CachePoolSnapshotsLimit)},
		persistStore:    node.persistStore,
		cosiActionsChan: make(chan *CosiAction, FinalPoolSlotsLimit),
		clc:             make(chan struct{}),
	}
	go func() {
		err := chain.ConsumeQueue()
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		err := chain.CosiLoop()
		if err != nil {
			panic(err)
		}
	}()
	return chain
}

func (chain *Chain) QueuePollSnapshots(hook func(peerId crypto.Hash, snap *common.Snapshot) error) {
	for {
		time.Sleep(1 * time.Millisecond)
		final, cache := 0, 0
		if round := chain.FinalPool[chain.FinalIndex]; round != nil {
			for _, ps := range round.Snapshots {
				hook(ps.PeerId, ps.Snapshot)
				if final > 10 {
					break
				}
				final++
			}
		}
		for i := 0; i < 2; i++ {
			item, err := chain.CachePool.Snapshots.Poll(false)
			if err != nil || item == nil {
				break
			}
			s := item.(*common.Snapshot)
			hook(s.NodeId, s)
			cache++
		}
		if cache < 1 && final < 1 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (chain *Chain) StepForward() {
	chain.FinalIndex = (chain.FinalIndex + 1) % FinalPoolSlotsLimit
}

func (ps *CosiAction) buildKey() crypto.Hash {
	ps.Snapshot.Hash = ps.Snapshot.PayloadHash()
	return ps.Snapshot.Hash.ForNetwork(ps.PeerId)
}

func (chain *Chain) AppendFinalSnapshot(peerId crypto.Hash, s *common.Snapshot) error {
	if s.NodeId != chain.ChainId {
		panic("final queue malformed")
	}
	start := chain.State.CacheRound
	if s.RoundNumber < start.Number { // FIXME initial accept
		return nil
	}
	offset := int(s.RoundNumber - start.Number)
	if offset >= FinalPoolSlotsLimit {
		return nil
	}
	offset = (offset + chain.FinalIndex) % FinalPoolSlotsLimit
	round := chain.FinalPool[offset]
	if round == nil || round.Number != s.RoundNumber {
		round = &ChainRound{
			NodeId:     chain.ChainId,
			Number:     s.RoundNumber,
			References: &common.RoundLink{Self: s.References.Self, External: s.References.External},
			finalSet:   make(map[crypto.Hash]bool),
		}
	}
	ps := &CosiAction{
		PeerId:   peerId,
		Snapshot: s,
	}
	ps.key = ps.buildKey()
	if round.finalSet[ps.key] {
		return nil
	}
	round.finalSet[ps.key] = true
	round.Snapshots = append(round.Snapshots, ps)
	chain.FinalPool[s.RoundNumber] = round
	return nil
}

func (chain *Chain) AppendCacheSnapshot(peerId crypto.Hash, s *common.Snapshot) error {
	if s.NodeId != chain.ChainId {
		panic("cache queue malformed")
	}
	if peerId != s.NodeId {
		panic("cache queue malformed")
	}
	if s.RoundNumber == 0 && s.NodeId != chain.node.IdForNetwork {
		return nil
	}
	if s.RoundNumber != 0 && s.NodeId == chain.node.IdForNetwork {
		return nil
	}
	if s.RoundNumber < chain.CachePool.Number {
		return nil
	}
	if s.RoundNumber > chain.CachePool.Number {
		chain.CachePool.Number = s.RoundNumber
		chain.CachePool.Snapshots.Dispose() // FIXME should reset the ring without init a new one
		chain.CachePool.Snapshots = NewRingBuffer(CachePoolSnapshotsLimit)
	}
	chain.CachePool.Snapshots.Offer(s)
	return nil
}

func (node *Node) GetOrCreateChain(id crypto.Hash) *Chain {
	chain := node.getChain(id)
	if chain != nil {
		return chain
	}

	node.chains.Lock()
	defer node.chains.Unlock()

	chain = node.chains.m[id]
	if chain != nil {
		return chain
	}

	node.chains.m[id] = node.BuildChain(id)
	return node.chains.m[id]
}

func (node *Node) getChain(id crypto.Hash) *Chain {
	node.chains.RLock()
	defer node.chains.RUnlock()
	return node.chains.m[id]
}

type chainsMap struct {
	sync.RWMutex
	m map[crypto.Hash]*Chain
}
