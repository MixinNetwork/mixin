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
	FinalPoolRoundSizeLimit = 1024
	CachePoolSnapshotsLimit = 1024
)

type PeerSnapshot struct {
	Snapshot *common.Snapshot
	peers    map[crypto.Hash]bool
}

type ChainRound struct {
	Number    uint64
	Size      int
	Snapshots [FinalPoolRoundSizeLimit]*PeerSnapshot
	index     map[crypto.Hash]int
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
	clc             chan struct{}
	plc             chan struct{}
	running         bool
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
		clc:             make(chan struct{}),
		plc:             make(chan struct{}),
		running:         true,
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

func (chain *Chain) Teardown() {
	chain.running = false
	<-chain.clc
	<-chain.plc
}

func (chain *Chain) UpdateState(cache *CacheRound, final *FinalRound, history []*FinalRound, reverseLinks map[crypto.Hash]uint64) {
	if chain.ChainId != cache.NodeId {
		panic("should never be here")
	}
	if chain.ChainId != final.NodeId {
		panic("should never be here")
	}

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
	defer close(chain.plc)

	for chain.running {
		time.Sleep(10 * time.Millisecond)
		final, cache := 0, 0
		for i := 0; i < 2; i++ {
			index := (chain.FinalIndex + i) % FinalPoolSlotsLimit
			round := chain.FinalPool[index]
			if round == nil {
				continue
			}
			if cr := chain.State.CacheRound; cr != nil && round.Number < cr.Number {
				continue
			}
			for i := 0; i < round.Size; i++ {
				ps := round.Snapshots[i]
				for k, _ := range ps.peers {
					hook(k, ps.Snapshot)
					final++
				}
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

func (chain *Chain) ClearFinalSnapshot(id crypto.Hash) error {
	chain.Lock()
	defer chain.Unlock()

	round := chain.FinalPool[chain.FinalIndex]
	if round == nil {
		return nil
	}
	index, found := round.index[id]
	if !found {
		return nil
	}
	round.Snapshots[index].peers = make(map[crypto.Hash]bool)
	return nil
}

func (chain *Chain) AppendFinalSnapshot(peerId crypto.Hash, s *common.Snapshot) error {
	if chain.ChainId != s.NodeId {
		panic("should never be here")
	}
	logger.Debugf("AppendFinalSnapshot(%s, %s)\n", peerId, s.Hash)
	if s.NodeId != chain.ChainId {
		panic("final queue malformed")
	}
	chain.Lock()
	defer chain.Unlock()

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
		return fmt.Errorf("AppendFinalSnapshot(%s, %s) pool slots full %d %d %d", peerId, s.Hash, start, s.RoundNumber, chain.FinalIndex)
	}
	offset = (offset + chain.FinalIndex) % FinalPoolSlotsLimit
	round := chain.FinalPool[offset]
	if round == nil {
		round = &ChainRound{
			Number: s.RoundNumber,
			index:  make(map[crypto.Hash]int),
		}
	}
	if round.Number != s.RoundNumber {
		round.Number = s.RoundNumber
		round.index = make(map[crypto.Hash]int)
		round.Size = 0
	}
	if round.Size == FinalPoolRoundSizeLimit {
		return fmt.Errorf("AppendFinalSnapshot(%s, %s) round snapshots full %s:%d", peerId, s.Hash, s.NodeId, s.RoundNumber)
	}
	index, found := round.index[s.Hash]
	if !found {
		round.Snapshots[round.Size] = &PeerSnapshot{
			Snapshot: s,
			peers:    map[crypto.Hash]bool{peerId: true},
		}
		round.Size = round.Size + 1
	} else {
		round.Snapshots[index].peers[peerId] = true
	}
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
	if chain.node.checkInitialAcceptSnapshotWeak(s) {
		chain.CachePool.Offer(s)
		return nil
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
