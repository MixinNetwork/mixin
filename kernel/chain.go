package kernel

import (
	"fmt"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
)

const (
	FinalPoolSlotsLimit     = config.SnapshotSyncRoundThreshold * 8
	FinalPoolRoundSizeLimit = 1024
	CachePoolSnapshotsLimit = 256
)

type PeerSnapshot struct {
	Snapshot  *common.Snapshot
	peers     map[crypto.Hash]bool
	finalized bool
}

type ChainRound struct {
	Number    uint64
	Size      int
	Timestamp uint64
	Snapshots [FinalPoolRoundSizeLimit]*PeerSnapshot
	index     map[crypto.Hash]int
}

type ChainState struct {
	CacheRound   *CacheRound
	FinalRound   *FinalRound
	RoundHistory []*FinalRound
	RoundLinks   map[crypto.Hash]uint64
}

type ActionBuffer chan *CosiAction

type Chain struct {
	sync.RWMutex
	node          *Node
	ChainId       crypto.Hash
	ConsensusInfo *CNode

	State *ChainState

	CosiAggregators map[crypto.Hash]*CosiAggregator
	CosiVerifiers   map[crypto.Hash]*CosiVerifier
	CachePool       ActionBuffer
	FinalPool       [FinalPoolSlotsLimit]*ChainRound
	FinalIndex      int
	FinalCount      int

	persistStore     storage.Store
	finalActionsRing ActionBuffer
	plc              chan struct{}
	clc              chan struct{}
	wlc              chan struct{}
	running          bool
}

func (node *Node) buildChain(chainId crypto.Hash) *Chain {
	chain := &Chain{
		node:             node,
		ChainId:          chainId,
		CosiAggregators:  make(map[crypto.Hash]*CosiAggregator),
		CosiVerifiers:    make(map[crypto.Hash]*CosiVerifier),
		CachePool:        make(chan *CosiAction, CachePoolSnapshotsLimit),
		persistStore:     node.persistStore,
		finalActionsRing: make(chan *CosiAction, FinalPoolSlotsLimit),
		plc:              make(chan struct{}),
		clc:              make(chan struct{}),
		wlc:              make(chan struct{}),
		running:          true,
	}

	err := chain.loadState()
	if err != nil {
		panic(err)
	}

	go chain.AggregateMintWork()
	go chain.QueuePollSnapshots()
	go chain.ConsumeFinalActions()
	return chain
}

func (ab ActionBuffer) Offer(m *CosiAction) error {
	select {
	case ab <- m:
		return nil
	default:
		return fmt.Errorf("full")
	}
}

func (ab ActionBuffer) Poll() *CosiAction {
	select {
	case m := <-ab:
		return m
	default:
		return nil
	}
}

func (chain *Chain) loadIdentity() *CNode {
	now := uint64(clock.Now().UnixNano())
	for _, n := range chain.node.NodesListWithoutState(now, false) {
		if chain.ChainId == n.IdForNetwork {
			return n
		}
	}
	if chain.node.IdForNetwork == chain.ChainId {
		return &CNode{
			IdForNetwork: chain.ChainId,
			Signer:       chain.node.Signer,
		}
	}
	return nil
}

func (chain *Chain) Teardown() {
	chain.running = false
	<-chain.clc
	<-chain.plc
	<-chain.wlc
}

func (chain *Chain) IsPledging() bool {
	return chain.State == nil && chain.ConsensusInfo != nil
}

func (chain *Chain) StateCopy() (*CacheRound, *FinalRound) {
	return chain.State.CacheRound.Copy(), chain.State.FinalRound.Copy()
}

func (chain *Chain) loadState() error {
	chain.Lock()
	defer chain.Unlock()

	if chain.State != nil {
		return nil
	}

	chain.ConsensusInfo = chain.loadIdentity()
	state := &ChainState{RoundLinks: make(map[crypto.Hash]uint64)}

	cache, err := loadHeadRoundForNode(chain.persistStore, chain.ChainId)
	if err != nil || cache == nil {
		return err
	}
	state.CacheRound = cache

	final, err := loadFinalRoundForNode(chain.persistStore, chain.ChainId, cache.Number-1)
	if err != nil {
		return err
	}
	state.FinalRound = final
	state.RoundHistory = loadRoundHistoryForNode(chain.persistStore, final)
	cache.Timestamp = final.Start + config.SnapshotRoundGap

	allNodes := chain.node.NodesListWithoutState(uint64(clock.Now().UnixNano()), false)
	for _, cn := range allNodes {
		if chain.ChainId == cn.IdForNetwork {
			continue
		}
		link, err := chain.persistStore.ReadLink(chain.ChainId, cn.IdForNetwork)
		if err != nil {
			return err
		}
		state.RoundLinks[cn.IdForNetwork] = link
	}

	chain.State = state
	return nil
}

func (chain *Chain) QueuePollSnapshots() {
	logger.Printf("QueuePollSnapshots(%s)\n", chain.ChainId)
	defer close(chain.plc)

	for chain.running {
		final, cache, stale := 0, 0, false
		for i := 0; i < 2; i++ {
			index := (chain.FinalIndex + i) % FinalPoolSlotsLimit
			round := chain.FinalPool[index]
			if round == nil {
				logger.Debugf("QueuePollSnapshots final round empty %s %d %d\n", chain.ChainId, chain.FinalIndex, index)
				continue
			}
			if cs := chain.State; cs != nil && (round.Number < cs.CacheRound.Number || round.Number > cs.CacheRound.Number+1) {
				logger.Debugf("QueuePollSnapshots final round number bad %s %d %d %d\n", chain.ChainId, chain.FinalIndex, cs.CacheRound.Number, round.Number)
				continue
			}
			if round.Timestamp > chain.node.GraphTimestamp+uint64(config.KernelNodeAcceptPeriodMaximum) {
				stale = true
			}
			logger.Debugf("QueuePollSnapshots final round good %s %d %d %d\n", chain.ChainId, chain.FinalIndex, round.Number, round.Size)
			for j := 0; j < round.Size; j++ {
				ps := round.Snapshots[j]
				logger.Debugf("QueuePollSnapshots final snapshot %s %d %s %t %d\n", chain.ChainId, chain.FinalIndex, ps.Snapshot.Hash, ps.finalized, len(ps.peers))
				if ps.finalized {
					continue
				}
				for pid := range ps.peers {
					finalized, err := chain.cosiHook(&CosiAction{
						PeerId:   pid,
						Action:   CosiActionFinalization,
						Snapshot: ps.Snapshot,
					})
					if err != nil {
						panic(err)
					}
					final++
					ps.finalized = finalized
					if ps.finalized {
						break
					}
				}
				if i != 0 {
					break
				}
			}
			logger.Debugf("QueuePollSnapshots final round done %s %d %d %d\n", chain.ChainId, chain.FinalIndex, round.Number, round.Size)
		}

		logger.Debugf("QueuePollSnapshots cache pool begin %s when final %d %d\n", chain.ChainId, chain.FinalIndex, chain.FinalCount)
		for {
			logger.Debugf("QueuePollSnapshots cache pool step %s from %d when final %d %d\n", chain.ChainId, cache, chain.FinalIndex, chain.FinalCount)
			m := chain.CachePool.Poll()
			if m == nil {
				logger.Verbosef("QueuePollSnapshots(%s) break at %d when final %d %d\n", chain.ChainId, cache, chain.FinalIndex, chain.FinalCount)
				break
			}
			logger.Debugf("QueuePollSnapshots cache pool step %s got %v when final %d %d\n", chain.ChainId, m, chain.FinalIndex, chain.FinalCount)
			_, err := chain.cosiHook(m)
			if err != nil {
				panic(err)
			}
			cache++
			logger.Debugf("QueuePollSnapshots cache pool step %s to %d when final %d %d\n", chain.ChainId, cache, chain.FinalIndex, chain.FinalCount)
			if cache > 256 {
				logger.Verbosef("QueuePollSnapshots(%s) break at %d when final %d %d\n", chain.ChainId, cache, chain.FinalIndex, chain.FinalCount)
				break
			}
		}
		logger.Debugf("QueuePollSnapshots cache pool end %s when final %d %d\n", chain.ChainId, chain.FinalIndex, chain.FinalCount)

		if stale || final == 0 && cache == 0 {
			time.Sleep(300 * time.Millisecond)
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (chain *Chain) StepForward() {
	logger.Debugf("graph chain StepForward(%d, %d)\n", chain.FinalIndex, chain.FinalCount)
	chain.FinalIndex = (chain.FinalIndex + 1) % FinalPoolSlotsLimit
	chain.FinalCount = chain.FinalCount + 1
}

func (chain *Chain) ConsumeFinalActions() {
	logger.Printf("ConsumeFinalActions(%s)\n", chain.ChainId)
	defer close(chain.clc)

	for chain.running {
		ps := chain.finalActionsRing.Poll()
		if ps == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		logger.Debugf("ConsumeFinalActions(%s) %s\n", chain.ChainId, ps.Snapshot.Hash)
		for chain.running {
			retry, err := chain.appendFinalSnapshot(ps.PeerId, ps.Snapshot)
			if err != nil {
				panic(err)
			} else if retry {
				time.Sleep(1 * time.Second)
			} else {
				break
			}
		}
	}
}

func (chain *Chain) appendFinalSnapshot(peerId crypto.Hash, s *common.Snapshot) (bool, error) {
	logger.Debugf("appendFinalSnapshot(%s, %s)\n", peerId, s.Hash)
	start, fi := uint64(0), chain.FinalIndex
	if chain.State != nil {
		start = chain.State.CacheRound.Number
		pr := chain.FinalPool[fi]
		if pr == nil || pr.Number == start || pr.Number+FinalPoolSlotsLimit == start {
			logger.Debugf("appendFinalSnapshot(%s, %s) cache and index match %d\n", peerId, s.Hash, start)
		} else {
			logger.Verbosef("appendFinalSnapshot(%s, %s) cache and index malformed %d %d\n", peerId, s.Hash, start, pr.Number)
			return true, nil
		}
	}
	if s.RoundNumber < start {
		logger.Debugf("appendFinalSnapshot(%s, %s) expired on start %d %d\n", peerId, s.Hash, s.RoundNumber, start)
		return false, nil
	}
	offset := int(s.RoundNumber - start)
	if offset >= FinalPoolSlotsLimit {
		logger.Verbosef("appendFinalSnapshot(%s, %s) pool slots full %d %d %d %d\n", peerId, s.Hash, start, s.RoundNumber, chain.FinalIndex, fi)
		return false, nil
	}
	offset = (offset + fi) % FinalPoolSlotsLimit
	round := chain.FinalPool[offset]
	if round == nil {
		round = &ChainRound{
			Number:    s.RoundNumber,
			Timestamp: s.Timestamp,
			index:     make(map[crypto.Hash]int),
			Size:      0,
		}
	} else if round.Number != s.RoundNumber {
		round.Number = s.RoundNumber
		round.Timestamp = s.Timestamp
		round.index = make(map[crypto.Hash]int)
		round.Size = 0
	}
	if round.Size == FinalPoolRoundSizeLimit {
		return false, fmt.Errorf("appendFinalSnapshot(%s, %s) round snapshots full %s:%d", peerId, s.Hash, s.NodeId, s.RoundNumber)
	}
	index, found := round.index[s.Hash]
	if !found {
		round.Snapshots[round.Size] = &PeerSnapshot{
			Snapshot: s,
			peers:    map[crypto.Hash]bool{peerId: true},
		}
		round.index[s.Hash] = round.Size
		round.Size = round.Size + 1
	} else {
		ps := round.Snapshots[index]
		if len(ps.peers) < 3 {
			ps.peers[peerId] = true
		}
	}
	chain.FinalPool[offset] = round
	return false, nil
}

func (chain *Chain) AppendFinalSnapshot(peerId crypto.Hash, s *common.Snapshot) error {
	logger.Debugf("AppendFinalSnapshot(%s, %s)\n", peerId, s.Hash)
	if s.NodeId != chain.ChainId {
		panic("final queue malformed")
	}
	if cs := chain.State; cs != nil && cs.CacheRound.Number > s.RoundNumber {
		return nil
	}
	ps := &CosiAction{PeerId: peerId, Snapshot: s}
	err := chain.finalActionsRing.Offer(ps)
	if err != nil {
		return fmt.Errorf("AppendFinalSnapshot(%s, %s) final actions ring full %d %d", peerId, s.Hash, s.RoundNumber, chain.FinalIndex)
	}
	return nil
}

func (chain *Chain) AppendCosiAction(m *CosiAction) error {
	logger.Debugf("AppendCosiAction(%s) %v\n", chain.ChainId, m)
	switch m.Action {
	case CosiActionSelfEmpty:
		if m.PeerId != chain.ChainId {
			panic("should never be here")
		}
		if chain.ChainId != chain.node.IdForNetwork {
			panic("should never be here")
		}
	case CosiActionSelfCommitment, CosiActionSelfResponse:
		if m.PeerId == chain.ChainId {
			panic("should never be here")
		}
		if chain.ChainId != chain.node.IdForNetwork {
			panic("should never be here")
		}
	case CosiActionExternalAnnouncement, CosiActionExternalChallenge:
		if m.PeerId != chain.ChainId {
			panic("should never be here")
		}
		if chain.ChainId == chain.node.IdForNetwork {
			panic("should never be here")
		}
	default:
		panic("should never be here")
	}

	err := chain.CachePool.Offer(m)
	if err != nil {
		logger.Verbosef("AppendCosiAction(%s) %v FULL\n", chain.ChainId, m)
	}
	return nil
}

func (chain *Chain) AppendSelfEmpty(s *common.Snapshot) error {
	return chain.AppendCosiAction(&CosiAction{
		PeerId:   chain.node.IdForNetwork,
		Action:   CosiActionSelfEmpty,
		Snapshot: s,
	})
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
	node.chains.m[id] = node.buildChain(id)
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
