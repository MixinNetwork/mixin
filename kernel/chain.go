package kernel

import (
	"fmt"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
)

const (
	FinalPoolSlotsLimit     = config.SnapshotSyncRoundThreshold * 8
	CachePoolSnapshotsLimit = 1024
)

type ChainRound struct {
	Number    uint64
	Snapshots []*CosiAction
	finalSet  map[crypto.Hash]bool
}

type ChainState struct {
	sync.RWMutex
	CacheRound        *CacheRound
	FinalRound        *FinalRound
	RoundHistory      []*FinalRound
	ReverseRoundLinks map[crypto.Hash]uint64
}

type Chain struct {
	sync.RWMutex
	node    *Node
	ChainId crypto.Hash

	State *ChainState

	CosiAggregators map[crypto.Hash]*CosiAggregator
	CosiVerifiers   map[crypto.Hash]*CosiVerifier
	CachePool       *RingBuffer
	CacheIndex      uint64
	FinalPool       [FinalPoolSlotsLimit]*ChainRound
	FinalIndex      int

	persistStore    storage.Store
	cosiActionsChan chan *CosiAction
}

func (node *Node) BuildChain(chainId crypto.Hash) *Chain {
	chain := &Chain{
		node:            node,
		ChainId:         chainId,
		State:           &ChainState{ReverseRoundLinks: make(map[crypto.Hash]uint64)},
		CosiAggregators: make(map[crypto.Hash]*CosiAggregator),
		CosiVerifiers:   make(map[crypto.Hash]*CosiVerifier),
		CachePool:       NewRingBuffer(CachePoolSnapshotsLimit),
		persistStore:    node.persistStore,
		cosiActionsChan: make(chan *CosiAction, FinalPoolSlotsLimit),
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

func (chain *Chain) UpdateState(cache *CacheRound, final *FinalRound, history []*FinalRound, reverseLinks map[crypto.Hash]uint64) {
	chain.State.Lock()
	defer chain.State.Unlock()

	chain.State.CacheRound = cache
	chain.State.FinalRound = final
	chain.State.RoundHistory = history
	chain.State.ReverseRoundLinks = reverseLinks

	if chain.ChainId == chain.node.IdForNetwork {
		chain.CacheIndex = 0
	} else if len(chain.State.CacheRound.Snapshots) == 0 {
		chain.CacheIndex = chain.State.CacheRound.Number
	} else {
		chain.CacheIndex = chain.State.CacheRound.Number + 1
	}
}

func (chain *Chain) QueuePollSnapshots(hook func(peerId crypto.Hash, snap *common.Snapshot) error) {
	for {
		time.Sleep(1 * time.Millisecond)
		final, cache := 0, 0
		for i := 0; i < 2; i++ {
			index := (chain.FinalIndex + i) % FinalPoolSlotsLimit
			round := chain.FinalPool[index]
			if round == nil {
				continue
			}
			for _, ps := range round.Snapshots {
				hook(ps.PeerId, ps.Snapshot)
				final++
			}
		}
		for i := 0; i < 16; i++ {
			item, err := chain.CachePool.Poll(false)
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
	logger.Debugf("AppendFinalSnapshot(%s, %s)\n", peerId, s.Hash)
	if s.NodeId != chain.ChainId {
		panic("final queue malformed")
	}
	start, offset := uint64(0), 0
	if chain.State.CacheRound != nil {
		start = chain.State.CacheRound.Number
	}
	if s.RoundNumber < start {
		logger.Debugf("AppendFinalSnapshot(%s, %s) expired %d %d\n", peerId, s.Hash, s.RoundNumber, start)
		return nil
	}
	offset = int(s.RoundNumber - start)
	if offset >= FinalPoolSlotsLimit {
		return fmt.Errorf("chain final pool slots full %d %d %d", start, s.RoundNumber, chain.FinalIndex)
	}
	offset = (offset + chain.FinalIndex) % FinalPoolSlotsLimit
	round := chain.FinalPool[offset]
	if round == nil || round.Number != s.RoundNumber {
		round = &ChainRound{
			Number:   s.RoundNumber,
			finalSet: make(map[crypto.Hash]bool),
		}
	}
	ps := &CosiAction{
		PeerId:   peerId,
		Snapshot: s,
	}
	chain.Lock()
	defer chain.Unlock()
	ps.key = ps.buildKey()
	if round.finalSet[ps.key] {
		return nil
	}
	round.finalSet[ps.key] = true
	round.Snapshots = append(round.Snapshots, ps)
	chain.FinalPool[offset] = round
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
	if s.RoundNumber < chain.CacheIndex {
		return nil
	}
	if s.RoundNumber > chain.CacheIndex {
		chain.CachePool.Dispose() // FIXME should reset the ring without init a new one
		chain.CacheIndex = s.RoundNumber
		chain.CachePool = NewRingBuffer(CachePoolSnapshotsLimit)
	}
	chain.CachePool.Offer(s)
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
