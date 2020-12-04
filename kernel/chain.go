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
	"github.com/MixinNetwork/mixin/util"
)

const (
	FinalPoolSlotsLimit     = config.SnapshotSyncRoundThreshold * 8
	FinalPoolRoundSizeLimit = 1024
	CachePoolSnapshotsLimit = 8192
)

type PeerSnapshot struct {
	Snapshot  *common.Snapshot
	filter    map[crypto.Hash]bool
	peers     []crypto.Hash
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
	sync.RWMutex
	CacheRound   *CacheRound
	FinalRound   *FinalRound
	RoundHistory []*FinalRound
	RoundLinks   map[crypto.Hash]uint64
}

type Chain struct {
	sync.RWMutex
	node          *Node
	ChainId       crypto.Hash
	ConsensusInfo *CNode

	State *ChainState

	CosiAggregators map[crypto.Hash]*CosiAggregator
	CosiVerifiers   map[crypto.Hash]*CosiVerifier
	CachePool       *util.RingBuffer
	FinalPool       [FinalPoolSlotsLimit]*ChainRound
	FinalIndex      int
	FinalCount      int

	persistStore     storage.Store
	finalActionsRing *util.RingBuffer
	plc              chan struct{}
	clc              chan struct{}
	running          bool
}

func (node *Node) BuildChain(chainId crypto.Hash) *Chain {
	chain := &Chain{
		node:    node,
		ChainId: chainId,
		State: &ChainState{
			RoundLinks: make(map[crypto.Hash]uint64),
		},
		CosiAggregators:  make(map[crypto.Hash]*CosiAggregator),
		CosiVerifiers:    make(map[crypto.Hash]*CosiVerifier),
		CachePool:        util.NewRingBuffer(CachePoolSnapshotsLimit),
		persistStore:     node.persistStore,
		finalActionsRing: util.NewRingBuffer(FinalPoolSlotsLimit),
		plc:              make(chan struct{}),
		clc:              make(chan struct{}),
		running:          true,
	}

	err := chain.loadState()
	if err != nil {
		panic(err)
	}

	go chain.QueuePollSnapshots()
	go chain.ConsumeFinalActions()
	return chain
}

func (node *Node) getConsensusInfo(id crypto.Hash) *CNode {
	for _, n := range node.allNodesSortedWithState {
		if id == n.IdForNetwork {
			return n
		}
	}
	if node.IdForNetwork == id {
		return &CNode{
			IdForNetwork: id,
			Signer:       node.Signer,
		}
	}
	return nil
}

func (chain *Chain) Teardown() {
	chain.running = false
	chain.CachePool.Dispose()
	chain.finalActionsRing.Dispose()
	<-chain.clc
	<-chain.plc
}

func (chain *Chain) IsPledging() bool {
	return chain.State.FinalRound == nil && chain.ConsensusInfo != nil
}

func (chain *Chain) StateCopy() (*CacheRound, *FinalRound) {
	return chain.State.CacheRound.Copy(), chain.State.FinalRound.Copy()
}

func (chain *Chain) loadState() error {
	chain.Lock()
	defer chain.Unlock()

	if chain.State.CacheRound != nil {
		return nil
	}
	chain.ConsensusInfo = chain.node.getConsensusInfo(chain.ChainId)

	cache, err := loadHeadRoundForNode(chain.persistStore, chain.ChainId)
	if err != nil || cache == nil {
		return err
	}
	chain.State.CacheRound = cache

	final, err := loadFinalRoundForNode(chain.persistStore, chain.ChainId, cache.Number-1)
	if err != nil {
		return err
	}
	history, err := loadRoundHistoryForNode(chain.persistStore, final)
	if err != nil {
		return err
	}
	chain.State.FinalRound = final
	chain.State.RoundHistory = history
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
		chain.State.RoundLinks[cn.IdForNetwork] = link
	}

	return nil
}

func (chain *Chain) QueuePollSnapshots() {
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
			cr := chain.State.CacheRound
			if cr != nil && (round.Number < cr.Number || round.Number > cr.Number+1) {
				logger.Debugf("QueuePollSnapshots final round number bad %s %d %d %d\n", chain.ChainId, chain.FinalIndex, cr.Number, round.Number)
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
				for _, pid := range ps.peers {
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
		}
		for i := 0; i < CachePoolSnapshotsLimit; i++ {
			item, err := chain.CachePool.Poll(false)
			if err != nil || item == nil {
				logger.Verbosef("QueuePollSnapshots(%s) break with %v\n", chain.ChainId, err)
				break
			}
			m := item.(*CosiAction)
			_, err = chain.cosiHook(m)
			if err != nil {
				panic(err)
			}
			cache++
		}
		if stale || final == 0 && cache == 0 {
			time.Sleep(100 * time.Millisecond)
		} else {
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func (chain *Chain) StepForward() {
	chain.FinalIndex = (chain.FinalIndex + 1) % FinalPoolSlotsLimit
	chain.FinalCount = chain.FinalCount + 1
}

func (chain *Chain) ConsumeFinalActions() {
	defer close(chain.clc)

	for chain.running {
		item, err := chain.finalActionsRing.Poll(false)
		if err != nil {
			logger.Verbosef("ConsumeFinalActions(%s) DONE %s\n", chain.ChainId, err)
			return
		} else if item == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		ps := item.(*CosiAction)
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
	if chain.State.CacheRound != nil {
		start = chain.State.CacheRound.Number
		pr := chain.FinalPool[fi]
		if pr == nil || pr.Number == start || pr.Number+FinalPoolSlotsLimit == start {
			logger.Debugf("AppendFinalSnapshot(%s, %s) cache and index match %d\n", peerId, s.Hash, start)
		} else {
			logger.Verbosef("AppendFinalSnapshot(%s, %s) cache and index malformed %d %d\n", peerId, s.Hash, start, pr.Number)
			return true, nil
		}
	}
	if s.RoundNumber < start {
		logger.Debugf("AppendFinalSnapshot(%s, %s) expired on start %d %d\n", peerId, s.Hash, s.RoundNumber, start)
		return false, nil
	}
	offset := int(s.RoundNumber - start)
	if offset >= FinalPoolSlotsLimit {
		logger.Verbosef("AppendFinalSnapshot(%s, %s) pool slots full %d %d %d %d\n", peerId, s.Hash, start, s.RoundNumber, chain.FinalIndex, fi)
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
		return false, fmt.Errorf("AppendFinalSnapshot(%s, %s) round snapshots full %s:%d", peerId, s.Hash, s.NodeId, s.RoundNumber)
	}
	index, found := round.index[s.Hash]
	if !found {
		round.Snapshots[round.Size] = &PeerSnapshot{
			Snapshot: s,
			filter:   map[crypto.Hash]bool{peerId: true},
			peers:    []crypto.Hash{peerId},
		}
		round.index[s.Hash] = round.Size
		round.Size = round.Size + 1
	} else {
		ps := round.Snapshots[index]
		if !ps.filter[peerId] {
			ps.filter[peerId] = true
			ps.peers = append(ps.peers, peerId)
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
	if cr := chain.State.CacheRound; cr != nil && cr.Number > s.RoundNumber {
		return nil
	}
	ps := &CosiAction{PeerId: peerId, Snapshot: s}
	success, _ := chain.finalActionsRing.Offer(ps)
	if !success {
		return fmt.Errorf("AppendFinalSnapshot(%s, %s) final actions ring full %d %d", peerId, s.Hash, s.RoundNumber, chain.FinalIndex)
	}
	return nil
}

func (chain *Chain) AppendCosiAction(m *CosiAction) error {
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

	_, err := chain.CachePool.Offer(m)
	if err != nil {
		// it is possible that the ring disposed, and this method is called concurrently
		logger.Verbosef("AppendCosiAction(%d, %s) ERROR %s\n", m.Action, m.SnapshotHash, err)
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
