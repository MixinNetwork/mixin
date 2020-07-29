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
	Snapshot  *common.Snapshot
	filter    map[crypto.Hash]bool
	peers     []crypto.Hash
	finalized bool
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
	RoundLinks        map[crypto.Hash]uint64
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

	persistStore     storage.Store
	finalActionsChan chan *CosiAction
	plc              chan struct{}
	running          bool
}

func (node *Node) BuildChain(chainId crypto.Hash) *Chain {
	chain := &Chain{
		node:    node,
		ChainId: chainId,
		State: &ChainState{
			RoundLinks:        make(map[crypto.Hash]uint64),
			ReverseRoundLinks: make(map[crypto.Hash]uint64),
		},
		CosiAggregators:  make(map[crypto.Hash]*CosiAggregator),
		CosiVerifiers:    make(map[crypto.Hash]*CosiVerifier),
		CachePool:        NewRingBuffer(CachePoolSnapshotsLimit),
		persistStore:     node.persistStore,
		finalActionsChan: make(chan *CosiAction, FinalPoolSlotsLimit),
		plc:              make(chan struct{}),
		running:          true,
	}

	err := chain.loadState(node.networkId, node.AllNodesSorted)
	if err != nil {
		panic(err)
	}

	go func() {
		err := chain.ConsumeQueue()
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		err := chain.consumeFinalActions()
		if err != nil {
			panic(err)
		}
	}()
	return chain
}

func (chain *Chain) Teardown() {
	chain.running = false
	<-chain.plc
}

func (chain *Chain) loadState(networkId crypto.Hash, allNodes []*common.Node) error {
	chain.Lock()
	defer chain.Unlock()

	if chain.State.CacheRound != nil {
		return nil
	}

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

	for _, cn := range allNodes {
		id := cn.IdForNetwork(networkId)
		if chain.ChainId == id {
			continue
		}
		link, err := chain.persistStore.ReadLink(chain.ChainId, id)
		if err != nil {
			return err
		}
		chain.State.RoundLinks[id] = link
		rlink, err := chain.persistStore.ReadLink(id, chain.ChainId)
		if err != nil {
			return err
		}
		chain.State.ReverseRoundLinks[id] = rlink
	}

	if chain.ChainId == chain.node.IdForNetwork {
		chain.CacheIndex = 0
	} else if len(chain.State.CacheRound.Snapshots) == 0 {
		chain.CacheIndex = chain.State.CacheRound.Number
	} else {
		chain.CacheIndex = chain.State.CacheRound.Number + 1
	}
	return nil
}

func (chain *Chain) QueuePollSnapshots(hook func(*CosiAction) (bool, error)) {
	defer close(chain.plc)

	for chain.running {
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
				if ps.finalized {
					continue
				}
				for _, pid := range ps.peers {
					m := &CosiAction{
						PeerId:   pid,
						Action:   CosiActionFinalization,
						Snapshot: ps.Snapshot,
					}
					finalized, err := hook(m)
					if err != nil {
						panic(err)
					} else if finalized {
						ps.finalized = true
						break
					}
					final++
				}
			}
		}
		for i := 0; i < CachePoolSnapshotsLimit; i++ {
			item, err := chain.CachePool.Poll(false)
			if err != nil || item == nil {
				break
			}
			m := item.(*CosiAction)
			_, err = hook(m)
			if err != nil {
				panic(err)
			}
			cache++
		}
		if final == 0 && cache == 0 {
			time.Sleep(100 * time.Millisecond)
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (chain *Chain) StepForward() {
	chain.FinalIndex = (chain.FinalIndex + 1) % FinalPoolSlotsLimit
}

func (chain *Chain) consumeFinalActions() error {
	for chain.running {
		select {
		case <-chain.node.done:
		case ps := <-chain.finalActionsChan:
			for chain.running {
				retry, err := chain.appendFinalSnapshot(ps.PeerId, ps.Snapshot)
				if err != nil {
					return err
				} else if retry {
					time.Sleep(1 * time.Second)
				} else {
					break
				}
			}
		}
	}
	return nil
}

func (chain *Chain) appendFinalSnapshot(peerId crypto.Hash, s *common.Snapshot) (bool, error) {
	start, offset := uint64(0), 0
	if chain.State.CacheRound != nil {
		start = chain.State.CacheRound.Number
		pr := chain.FinalPool[chain.FinalIndex]
		if pr != nil && pr.Number != start {
			logger.Verbosef("AppendFinalSnapshot(%s, %s) cache and index malformed %d %d\n", peerId, s.Hash, start, pr.Number)
			return true, nil
		}
	}
	if s.RoundNumber < start {
		logger.Debugf("AppendFinalSnapshot(%s, %s) expired %d %d\n", peerId, s.Hash, s.RoundNumber, start)
		return false, nil
	}
	offset = int(s.RoundNumber - start)
	if offset >= FinalPoolSlotsLimit {
		logger.Verbosef("AppendFinalSnapshot(%s, %s) pool slots full %d %d %d\n", peerId, s.Hash, start, s.RoundNumber, chain.FinalIndex)
		return true, nil
	}
	offset = (offset + chain.FinalIndex) % FinalPoolSlotsLimit
	round := chain.FinalPool[offset]
	if round == nil {
		round = &ChainRound{
			Number: s.RoundNumber,
			index:  make(map[crypto.Hash]int),
			Size:   0,
		}
	} else if round.Number != s.RoundNumber {
		round.Number = s.RoundNumber
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
	ps := &CosiAction{PeerId: peerId, Snapshot: s}
	select {
	case chain.finalActionsChan <- ps:
		return nil
	default:
		return fmt.Errorf("AppendFinalSnapshot(%s, %s) pool slots full %d %d", peerId, s.Hash, s.RoundNumber, chain.FinalIndex)
	}
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
	case CosiActionSelfCommitment:
		if m.PeerId == chain.ChainId {
			panic("should never be here")
		}
		if chain.ChainId != chain.node.IdForNetwork {
			panic("should never be here")
		}
	case CosiActionSelfResponse:
		if m.PeerId == chain.ChainId {
			panic("should never be here")
		}
		if chain.ChainId != chain.node.IdForNetwork {
			panic("should never be here")
		}
	case CosiActionExternalAnnouncement:
		if m.PeerId != chain.ChainId {
			panic("should never be here")
		}
		if chain.ChainId == chain.node.IdForNetwork {
			panic("should never be here")
		}
	case CosiActionExternalChallenge:
		if m.PeerId != chain.ChainId {
			panic("should never be here")
		}
		if chain.ChainId == chain.node.IdForNetwork {
			panic("should never be here")
		}
	default:
		panic("should never be here")
	}

	if s := m.Snapshot; s != nil {
		if s.NodeId != chain.ChainId {
			panic("should never be here")
		}
		if s.RoundNumber < chain.CacheIndex {
			return nil
		}
		if s.RoundNumber > chain.CacheIndex {
			chain.CachePool.Reset()
			chain.CacheIndex = s.RoundNumber
		}
	}

	_, err := chain.CachePool.Offer(m)
	return err
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
