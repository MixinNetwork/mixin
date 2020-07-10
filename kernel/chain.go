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
	ChainRoundSlotsSize      = config.SnapshotSyncRoundThreshold * 8
	ChainCacheSnapshotsCount = 1024
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

type Chain struct {
	node    *Node
	ChainId crypto.Hash

	CacheRound  *ChainCache
	FinalRounds [ChainRoundSlotsSize]*ChainRound
	FinalIndex  int

	CosiAggregators map[crypto.Hash]*CosiAggregator
	CosiVerifiers   map[crypto.Hash]*CosiVerifier

	persistStore    storage.Store
	cosiActionsChan chan *CosiAction

	clc chan struct{}
}

func (node *Node) BuildChain(chainId crypto.Hash) *Chain {
	chain := &Chain{
		node:            node,
		ChainId:         chainId,
		CacheRound:      &ChainCache{NodeId: chainId, Snapshots: NewRingBuffer(ChainCacheSnapshotsCount)},
		CosiAggregators: make(map[crypto.Hash]*CosiAggregator),
		CosiVerifiers:   make(map[crypto.Hash]*CosiVerifier),
		persistStore:    node.persistStore,
		cosiActionsChan: make(chan *CosiAction, ChainRoundSlotsSize),
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
		if round := chain.FinalRounds[chain.FinalIndex]; round != nil {
			for _, ps := range round.Snapshots {
				hook(ps.PeerId, ps.Snapshot)
				if final > 10 {
					break
				}
				final++
			}
		}
		for i := 0; i < 2; i++ {
			item, err := chain.CacheRound.Snapshots.Poll(false)
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

func (chain *Chain) TryToStepForward(roundHash crypto.Hash) {
	next := (chain.FinalIndex + 1) % ChainRoundSlotsSize
	round := chain.FinalRounds[next]
	if round != nil && round.References.Self == roundHash {
		chain.FinalIndex = next
	}
}

func (ps *CosiAction) buildKey() crypto.Hash {
	ps.Snapshot.Hash = ps.Snapshot.PayloadHash()
	return ps.Snapshot.Hash.ForNetwork(ps.PeerId)
}

func (chain *Chain) AppendFinalSnapshot(peerId crypto.Hash, s *common.Snapshot) error {
	if s.NodeId != chain.ChainId {
		panic("final queue malformed")
	}
	start := chain.FinalRounds[chain.FinalIndex]
	diff := s.RoundNumber - start.Number
	if diff > ChainRoundSlotsSize {
		return nil
	}
	round := chain.FinalRounds[s.RoundNumber]
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
	chain.FinalRounds[s.RoundNumber] = round
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
	if s.RoundNumber < chain.CacheRound.Number {
		return nil
	}
	if s.RoundNumber > chain.CacheRound.Number {
		chain.CacheRound.Number = s.RoundNumber
		chain.CacheRound.Snapshots.Dispose()
		chain.CacheRound.Snapshots = NewRingBuffer(ChainCacheSnapshotsCount)
	}
	chain.CacheRound.Snapshots.Offer(s)
	return nil
}

func (node *Node) GetOrCreateChain(id crypto.Hash) *Chain {
	chain := node.getChain(id)
	if chain != nil {
		return chain
	}

	node.chains.mutex.Lock()
	defer node.chains.mutex.Unlock()

	chain = node.chains.m[id]
	if chain != nil {
		return chain
	}

	node.chains.m[id] = node.BuildChain(id)
	return node.chains.m[id]
}

func (node *Node) getChain(id crypto.Hash) *Chain {
	node.chains.mutex.RLock()
	defer node.chains.mutex.RUnlock()
	return node.chains.m[id]
}

type chainsMap struct {
	mutex *sync.RWMutex
	m     map[crypto.Hash]*Chain
}
