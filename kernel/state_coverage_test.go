package kernel

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/p2p"
	"github.com/stretchr/testify/require"
)

func TestCacheRoundValidationInvariants(t *testing.T) {
	nodeID := crypto.Blake3Hash([]byte("round node"))
	base := uint64(time.Hour)
	gap := config.SnapshotRoundGap
	newSnapshot := func(label string, round, timestamp uint64, transactions ...crypto.Hash) *common.Snapshot {
		return &common.Snapshot{
			Version:      common.SnapshotVersionCommonEncoding,
			NodeId:       nodeID,
			RoundNumber:  round,
			Timestamp:    timestamp,
			Hash:         crypto.Blake3Hash([]byte(label)),
			Transactions: transactions,
		}
	}

	t.Run("copy and final round conversion", func(t *testing.T) {
		references := &common.RoundLink{
			Self:     crypto.Blake3Hash([]byte("self")),
			External: crypto.Blake3Hash([]byte("external")),
		}
		first := newSnapshot("first", 7, base+gap/2)
		round := &CacheRound{
			NodeId:     nodeID,
			Number:     7,
			Timestamp:  base,
			References: references,
			Snapshots:  []*common.Snapshot{first},
			index:      newRoundIndexCache(),
		}

		copy := round.Copy()
		require.Equal(t, round, copy)
		require.NotSame(t, round.References, copy.References)
		copy.References.Self = crypto.Hash{}
		copy.Snapshots[0] = newSnapshot("replacement", 7, base)
		require.True(t, round.References.Self.HasValue())
		require.Same(t, first, round.Snapshots[0])

		final := round.asFinal()
		require.NotNil(t, final)
		require.Equal(t, nodeID, final.NodeId)
		require.Equal(t, uint64(7), final.Number)
		require.Equal(t, first.Timestamp, final.Start)
		require.Equal(t, first.Timestamp, final.End)
		require.True(t, final.Hash.HasValue())

		finalCopy := final.Copy()
		require.Equal(t, final, finalCopy)
		require.NotSame(t, final, finalCopy)
		require.Equal(t, &common.Round{
			Hash:      final.Hash,
			NodeId:    nodeID,
			Number:    7,
			Timestamp: first.Timestamp,
		}, final.Common())
		require.Nil(t, (&CacheRound{}).asFinal())
	})

	t.Run("gap bounds", func(t *testing.T) {
		empty := &CacheRound{}
		start, end := empty.Gap()
		require.Equal(t, (^uint64(0))/2, start)
		require.Zero(t, end)

		late := newSnapshot("late", 4, base+gap/2)
		early := newSnapshot("early", 4, base)
		round := &CacheRound{NodeId: nodeID, Number: 4, Snapshots: []*common.Snapshot{late, early}}
		start, end = round.Gap()
		require.Equal(t, base, start)
		require.Equal(t, base+gap/2, end)
		require.Same(t, early, round.Snapshots[0])

		invalid := &CacheRound{NodeId: nodeID, Number: 4, Snapshots: []*common.Snapshot{
			newSnapshot("gap start", 4, base),
			newSnapshot("gap end", 4, base+gap),
		}}
		require.Panics(t, func() { invalid.Gap() })
	})

	t.Run("validation errors and append", func(t *testing.T) {
		tx1 := crypto.Blake3Hash([]byte("transaction one"))
		tx2 := crypto.Blake3Hash([]byte("transaction two"))
		first := newSnapshot("existing", 9, base+gap/3, tx1)
		round := &CacheRound{NodeId: nodeID, Number: 9, Snapshots: []*common.Snapshot{first}}

		candidate := newSnapshot("candidate", 9, base+gap/2, tx2)
		require.NoError(t, round.ValidateSnapshot(candidate))
		require.Len(t, round.Snapshots, 1)
		require.NoError(t, round.validateSnapshot(candidate, true))
		require.Len(t, round.Snapshots, 2)

		duplicateHash := newSnapshot("unused", 9, base+gap*2/3)
		duplicateHash.Hash = candidate.Hash
		require.ErrorContains(t, round.ValidateSnapshot(duplicateHash), "duplication")
		require.ErrorContains(t, round.ValidateSnapshot(newSnapshot("duplicate timestamp", 9, candidate.Timestamp)), "duplication")
		require.ErrorContains(t, round.ValidateSnapshot(newSnapshot("day leap", 9, base+OneDay)), "day leap")
		require.ErrorContains(t, round.ValidateSnapshot(newSnapshot("duplicate transaction", 9, base+gap*3/4, tx1)), "duplication")

		before := &CacheRound{NodeId: nodeID, Number: 9, Snapshots: []*common.Snapshot{
			newSnapshot("later existing", 9, base+gap),
		}}
		require.ErrorContains(t, before.ValidateSnapshot(newSnapshot("too early", 9, base)), "gap start")
		after := &CacheRound{NodeId: nodeID, Number: 9, Snapshots: []*common.Snapshot{
			newSnapshot("earlier existing", 9, base),
		}}
		require.ErrorContains(t, after.ValidateSnapshot(newSnapshot("too late", 9, base+gap)), "gap end")

		require.Panics(t, func() {
			round.ValidateSnapshot(newSnapshot("wrong round", 10, base+gap/2))
		})
		missingHash := newSnapshot("missing hash", 9, base+gap/2)
		missingHash.Hash = crypto.Hash{}
		require.Panics(t, func() { round.ValidateSnapshot(missingHash) })
	})

	t.Run("snapshot index", func(t *testing.T) {
		index := newRoundIndexCache()
		hash := crypto.Blake3Hash([]byte("indexed snapshot"))
		require.False(t, index.Check(hash))
		index.Store(hash)
		require.True(t, index.Check(hash))
	})
}

func TestChainQueueBoundaries(t *testing.T) {
	localID := crypto.Blake3Hash([]byte("local chain"))
	remoteID := crypto.Blake3Hash([]byte("remote chain"))
	peer := func(label string) crypto.Hash { return crypto.Blake3Hash([]byte(label)) }
	snapshot := func(label string, round uint64) *common.Snapshot {
		return &common.Snapshot{
			Version:     common.SnapshotVersionCommonEncoding,
			NodeId:      localID,
			RoundNumber: round,
			Timestamp:   uint64(time.Hour) + round,
			Hash:        crypto.Blake3Hash([]byte(label)),
		}
	}
	newChain := func() *Chain {
		node := &Node{IdForNetwork: localID, done: make(chan struct{})}
		return &Chain{
			node:             node,
			ChainId:          localID,
			CachePool:        make(ActionBuffer, 1),
			finalActionsRing: make(ActionBuffer, 1),
		}
	}

	t.Run("action buffer", func(t *testing.T) {
		buffer := make(ActionBuffer, 1)
		message := &CosiAction{Action: CosiActionSelfEmpty}
		require.Nil(t, buffer.Poll())
		require.NoError(t, buffer.Offer(message))
		require.Error(t, buffer.Offer(&CosiAction{}))
		require.Same(t, message, buffer.Poll())
		require.Nil(t, buffer.Poll())
	})

	t.Run("state helpers", func(t *testing.T) {
		chain := newChain()
		require.False(t, chain.IsPledging())
		chain.ConsensusInfo = &CNode{IdForNetwork: localID}
		require.True(t, chain.IsPledging())
		chain.State = &ChainState{
			CacheRound: &CacheRound{
				NodeId:     localID,
				Number:     2,
				References: &common.RoundLink{},
			},
			FinalRound: &FinalRound{NodeId: localID, Number: 1},
		}
		cache, final := chain.StateCopy()
		require.NotSame(t, chain.State.CacheRound, cache)
		require.NotSame(t, chain.State.FinalRound, final)

		chain.FinalIndex = FinalPoolSlotsLimit - 1
		chain.FinalCount = 7
		chain.StepForward()
		require.Zero(t, chain.FinalIndex)
		require.Equal(t, 8, chain.FinalCount)

		chain.running = true
		close(chain.node.done)
		chain.waitOrDone(time.Hour)
		require.False(t, chain.running)
	})

	t.Run("final snapshot pool", func(t *testing.T) {
		chain := newChain()
		s := snapshot("pooled", 0)
		for _, id := range []crypto.Hash{peer("one"), peer("two"), peer("three"), peer("four"), peer("one")} {
			retry, err := chain.appendFinalSnapshot(id, s)
			require.False(t, retry)
			require.NoError(t, err)
		}
		round := chain.FinalPool[0]
		require.Equal(t, 1, round.Size)
		require.Len(t, round.Snapshots[0].peers, 3)

		chain.FinalPool[1] = &ChainRound{Number: 99, Size: 3, index: map[crypto.Hash]int{}}
		retry, err := chain.appendFinalSnapshot(peer("reset"), snapshot("reset", 1))
		require.False(t, retry)
		require.NoError(t, err)
		require.Equal(t, uint64(1), chain.FinalPool[1].Number)
		require.Equal(t, 1, chain.FinalPool[1].Size)

		chain.FinalPool[2] = &ChainRound{
			Number: 2,
			Size:   FinalPoolRoundSizeLimit,
			index:  make(map[crypto.Hash]int),
		}
		retry, err = chain.appendFinalSnapshot(peer("full"), snapshot("full", 2))
		require.False(t, retry)
		require.ErrorContains(t, err, "round snapshots full")

		stateful := newChain()
		stateful.State = &ChainState{CacheRound: &CacheRound{Number: 10}}
		stateful.FinalPool[0] = &ChainRound{Number: 5}
		retry, err = stateful.appendFinalSnapshot(peer("malformed"), snapshot("malformed", 10))
		require.True(t, retry)
		require.NoError(t, err)
		stateful.FinalPool[0] = nil
		retry, err = stateful.appendFinalSnapshot(peer("expired"), snapshot("expired", 9))
		require.False(t, retry)
		require.NoError(t, err)
		retry, err = stateful.appendFinalSnapshot(peer("outside"), snapshot("outside", 10+FinalPoolSlotsLimit))
		require.False(t, retry)
		require.NoError(t, err)
	})

	t.Run("public queue guards", func(t *testing.T) {
		chain := newChain()
		wrongNode := snapshot("wrong node", 0)
		wrongNode.NodeId = remoteID
		require.Panics(t, func() { chain.AppendFinalSnapshot(peer("peer"), wrongNode) })

		chain.State = &ChainState{CacheRound: &CacheRound{Number: 2}}
		require.NoError(t, chain.AppendFinalSnapshot(peer("old"), snapshot("old", 1)))
		require.Empty(t, chain.finalActionsRing)
		require.NoError(t, chain.AppendFinalSnapshot(peer("queued"), snapshot("queued", 2)))
		require.ErrorContains(t, chain.AppendFinalSnapshot(peer("full"), snapshot("full queue", 2)), "ring full")

		actionChain := newChain()
		require.NoError(t, actionChain.AppendCosiAction(&CosiAction{Action: CosiActionSelfEmpty, PeerId: localID}))
		// Cache pressure is intentionally lossy; callers should never block on it.
		require.NoError(t, actionChain.AppendCosiAction(&CosiAction{Action: CosiActionSelfEmpty, PeerId: localID}))
		require.Panics(t, func() {
			actionChain.AppendCosiAction(&CosiAction{Action: CosiActionSelfEmpty, PeerId: remoteID})
		})
		require.Panics(t, func() {
			actionChain.AppendCosiAction(&CosiAction{Action: CosiActionSelfCommitment, PeerId: localID})
		})

		external := newChain()
		external.ChainId = remoteID
		require.NoError(t, external.AppendCosiAction(&CosiAction{
			Action: CosiActionExternalAnnouncement,
			PeerId: remoteID,
		}))
		require.Panics(t, func() {
			external.AppendCosiAction(&CosiAction{Action: CosiActionExternalChallenge, PeerId: localID})
		})
		require.Panics(t, func() {
			external.AppendCosiAction(&CosiAction{Action: CosiActionSelfResponse, PeerId: remoteID})
		})
		require.Panics(t, func() {
			external.AppendCosiAction(&CosiAction{Action: -1, PeerId: remoteID})
		})

		require.Panics(t, func() { actionChain.AppendSelfEmpty(snapshot("empty", 0)) })
		withTransaction := snapshot("self", 0)
		withTransaction.AddTransaction(crypto.Blake3Hash([]byte("self transaction")))
		fresh := newChain()
		require.NoError(t, fresh.AppendSelfEmpty(withTransaction))
	})
}

func TestGraphHistoryAndReferenceInvariants(t *testing.T) {
	t.Run("history reduction", func(t *testing.T) {
		closeRounds := []*FinalRound{{Number: 1, Start: 100}, {Number: 2, Start: 200}}
		unchanged := reduceHistory(closeRounds)
		require.Len(t, unchanged, 2)
		require.Same(t, closeRounds[0], unchanged[0])

		threshold := config.SnapshotReferenceThreshold * config.SnapshotRoundGap * 64
		oldAndNew := []*FinalRound{{Number: 1, Start: 1}, {Number: 2, Start: 1 + threshold}}
		reduced := reduceHistory(oldAndNew)
		require.Len(t, reduced, 1)
		require.Equal(t, uint64(2), reduced[0].Number)

		many := make([]*FinalRound, config.SnapshotReferenceThreshold+2)
		for i := range many {
			many[i] = &FinalRound{Number: uint64(i), Start: uint64(i)}
		}
		reduced = reduceHistory(many)
		require.Len(t, reduced, config.SnapshotReferenceThreshold)
		require.Equal(t, uint64(2), reduced[0].Number)

		require.Equal(t, many[5:], historySinceRound(many, 5))
		require.Nil(t, historySinceRound(many, uint64(len(many)+1)))
	})

	t.Run("signer encoding", func(t *testing.T) {
		signers := []crypto.Hash{
			crypto.Blake3Hash([]byte("signer zero")),
			crypto.Blake3Hash([]byte("signer two")),
		}
		signature := &crypto.CosiSignature{Mask: 1 | 1<<2}
		encoded := convertSignersToBytes(signers)
		require.Len(t, encoded, 64)
		require.Equal(t, signers, convertBytesToSigners(signature, encoded))
		require.Nil(t, convertBytesToSigners(signature, encoded[:63]))
	})

	t.Run("reference sanity", func(t *testing.T) {
		localID := crypto.Blake3Hash([]byte("reference local"))
		externalID := crypto.Blake3Hash([]byte("reference external"))
		node := &Node{genesisNodesMap: make(map[crypto.Hash]bool)}
		chain := &Chain{node: node, ChainId: localID}
		externalChain := &Chain{State: &ChainState{
			CacheRound: &CacheRound{Number: 21, Snapshots: []*common.Snapshot{{}}},
			FinalRound: &FinalRound{Number: 20, Start: 1},
		}}

		external := &common.Round{NodeId: externalID, Number: 20, Timestamp: 200}
		require.ErrorContains(t, chain.checkReferenceSanity(externalChain, external, 100), "later than snapshot")
		external.Timestamp = 100
		external.Number = 1
		require.ErrorContains(t, chain.checkReferenceSanity(externalChain, external, 200), "not genesis")

		node.genesisNodesMap[externalID] = true
		external.Number = 20
		externalChain.State.FinalRound.Start = (^uint64(0)) / 2
		require.ErrorContains(t, chain.checkReferenceSanity(externalChain, external, 200), "too future")

		externalChain.State.FinalRound.Start = 1
		externalChain.State.CacheRound.Snapshots = nil
		require.ErrorContains(t, chain.checkReferenceSanity(externalChain, external, 200), "without extra final")
		externalChain.State.CacheRound.Snapshots = []*common.Snapshot{{}}
		require.NoError(t, chain.checkReferenceSanity(externalChain, external, 200))
	})

	t.Run("empty head transition guards", func(t *testing.T) {
		chain := &Chain{}
		cache := &CacheRound{Snapshots: []*common.Snapshot{{}}}
		references := &common.RoundLink{}
		require.ErrorContains(t, chain.updateEmptyHeadRoundAndPersist(nil, cache, references, 0, false), "references not empty")

		cache.Snapshots = nil
		cache.References = &common.RoundLink{Self: crypto.Blake3Hash([]byte("old self"))}
		references.Self = crypto.Blake3Hash([]byte("new self"))
		require.ErrorContains(t, chain.updateEmptyHeadRoundAndPersist(nil, cache, references, 0, false), "references self diff")
	})

	t.Run("new round and finalization guards", func(t *testing.T) {
		localID := crypto.Blake3Hash([]byte("new round local"))
		otherID := crypto.Blake3Hash([]byte("new round other"))
		chain := &Chain{ChainId: localID}
		wrongChain := &CacheRound{NodeId: otherID}
		require.Panics(t, func() {
			chain.validateNewRound(wrongChain, &common.RoundLink{}, 0, false)
		})

		empty := &CacheRound{NodeId: localID, Number: 3}
		final, dummy, err := chain.validateNewRound(empty, &common.RoundLink{}, 0, false)
		require.Nil(t, final)
		require.False(t, dummy)
		require.ErrorContains(t, err, "snapshots not collected")

		collected := &CacheRound{NodeId: localID, Number: 3, Snapshots: []*common.Snapshot{{
			Version:     common.SnapshotVersionCommonEncoding,
			NodeId:      localID,
			RoundNumber: 3,
			Timestamp:   100,
			Hash:        crypto.Blake3Hash([]byte("collected snapshot")),
		}}}
		final, dummy, err = chain.validateNewRound(collected, &common.RoundLink{}, 0, false)
		require.Nil(t, final)
		require.False(t, dummy)
		require.ErrorContains(t, err, "snapshots not match")

		signers, finalized := chain.verifyFinalization(&common.Snapshot{Version: 1})
		require.Nil(t, signers)
		require.False(t, finalized)
		signers, finalized = chain.verifyFinalization(&common.Snapshot{Version: common.SnapshotVersionCommonEncoding})
		require.Nil(t, signers)
		require.False(t, finalized)
	})
}

func TestElectionTransitionGuards(t *testing.T) {
	chainID := crypto.Blake3Hash([]byte("pledging chain"))
	info := &CNode{IdForNetwork: chainID, State: common.NodeStatePledging}
	node := &Node{Epoch: 100}
	chain := &Chain{
		node:          node,
		ChainId:       chainID,
		ConsensusInfo: info,
		State:         &ChainState{CacheRound: &CacheRound{Number: 4}},
	}
	require.ErrorContains(t, chain.checkNodeAcceptPossibility(50, false), "invalid graph round")

	chain.State = nil
	require.ErrorContains(t, chain.checkNodeAcceptPossibility(50, false), "no consensus pledging node")

	node.nodeStateSequences = []*NodeStateSequence{{
		Timestamp:         1,
		NodesWithoutState: []*CNode{info},
	}}
	require.ErrorContains(t, chain.checkNodeAcceptPossibility(50, false), "invalid snapshot timestamp")

	node.Epoch = 1
	require.False(t, node.checkConsensusAcceptHour(node.Epoch+uint64(8*time.Hour)))
	require.True(t, node.checkConsensusAcceptHour(node.Epoch+uint64(14*time.Hour)))
	require.False(t, node.checkConsensusPledgeHour(node.Epoch+uint64(8*time.Hour)))
	require.False(t, node.checkConsensusPledgeHour(node.Epoch+uint64(14*time.Hour)))
	require.True(t, node.checkConsensusPledgeHour(node.Epoch+uint64(11*time.Hour)))

	t.Run("node accept requires the initial round", func(t *testing.T) {
		node := &Node{IdForNetwork: chainID}
		snapshot := &common.Snapshot{NodeId: chainID, RoundNumber: 1, Timestamp: 1}
		require.ErrorContains(t, node.validateNodeAcceptSnapshot(snapshot, nil, false), "invalid snapshot round")
	})

	t.Run("node cancellation timing", func(t *testing.T) {
		cancelSnapshot := func(timestamp uint64) *common.Snapshot {
			return &common.Snapshot{NodeId: chainID, Timestamp: timestamp}
		}
		withPledgingNode := func(epoch, graphTimestamp, pledgingTimestamp uint64) *Node {
			pledging := &CNode{
				IdForNetwork: chainID,
				State:        common.NodeStatePledging,
				Timestamp:    pledgingTimestamp,
			}
			return &Node{
				Epoch:          epoch,
				GraphTimestamp: graphTimestamp,
				nodeStateSequences: []*NodeStateSequence{{
					Timestamp:         1,
					NodesWithoutState: []*CNode{pledging},
				}},
			}
		}

		node := &Node{Epoch: 100}
		require.ErrorContains(t, node.validateNodeCancelSnapshot(cancelSnapshot(50), nil, false), "invalid snapshot timestamp")

		timestamp := uint64(14 * time.Hour)
		node = &Node{}
		require.ErrorContains(t, node.validateNodeCancelSnapshot(cancelSnapshot(timestamp), nil, false), "invalid consensus status")

		node = withPledgingNode(0, 0, 1)
		require.ErrorContains(t, node.validateNodeCancelSnapshot(cancelSnapshot(uint64(11*time.Hour)), nil, false), "invalid node cancel hour")

		staleGraph := timestamp + config.SnapshotRoundGap*config.SnapshotReferenceThreshold*2 + 1
		node = withPledgingNode(0, staleGraph, 1)
		require.ErrorContains(t, node.validateNodeCancelSnapshot(cancelSnapshot(timestamp), nil, false), "invalid snapshot timestamp")

		node = withPledgingNode(0, 0, timestamp+1)
		require.ErrorContains(t, node.validateNodeCancelSnapshot(cancelSnapshot(timestamp), nil, false), "invalid snapshot timestamp")

		node = withPledgingNode(0, 0, timestamp-uint64(time.Hour))
		require.ErrorContains(t, node.validateNodeCancelSnapshot(cancelSnapshot(timestamp), nil, false), "invalid cancel period")

		lateTimestamp := uint64(10*24*time.Hour + 14*time.Hour)
		node = withPledgingNode(0, 0, uint64(time.Hour))
		require.ErrorContains(t, node.validateNodeCancelSnapshot(cancelSnapshot(lateTimestamp), nil, false), "invalid cancel period")
	})
}

func TestKernelSnapshotBatchGuards(t *testing.T) {
	localID := crypto.Blake3Hash([]byte("kernel snapshot local"))
	remoteID := crypto.Blake3Hash([]byte("kernel snapshot remote"))
	node := &Node{IdForNetwork: localID}

	first := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	second := common.NewTransactionV5(crypto.Blake3Hash([]byte("second asset"))).AsVersioned()
	batch := &common.Snapshot{
		NodeId:      localID,
		RoundNumber: 1,
		Timestamp:   1,
		Transactions: []crypto.Hash{
			first.PayloadHash(),
			second.PayloadHash(),
		},
	}
	found := map[crypto.Hash]*common.VersionedTransaction{
		first.PayloadHash():  first,
		second.PayloadHash(): second,
	}
	require.NoError(t, node.validateKernelSnapshot(batch, found, false))

	nodeAccept := common.NewTransactionV5(common.XINAssetId)
	nodeAccept.AddOutputWithType(common.OutputTypeNodeAccept, nil, common.Script{}, common.NewInteger(1), nil)
	accept := nodeAccept.AsVersioned()
	batch.Transactions[1] = accept.PayloadHash()
	found = map[crypto.Hash]*common.VersionedTransaction{
		first.PayloadHash():  first,
		accept.PayloadHash(): accept,
	}
	require.ErrorContains(t, node.validateKernelSnapshot(batch, found, false), "non batchable")

	initial := &common.Snapshot{
		NodeId:      remoteID,
		RoundNumber: 0,
		Timestamp:   1,
		Transactions: []crypto.Hash{
			first.PayloadHash(),
		},
	}
	found = map[crypto.Hash]*common.VersionedTransaction{first.PayloadHash(): first}
	require.ErrorContains(t, node.validateKernelSnapshot(initial, found, false), "invalid initial transaction type")
	initial.NodeId = localID
	require.NoError(t, node.validateKernelSnapshot(initial, found, false))
}

func TestCosiActionSanityGuards(t *testing.T) {
	localID := crypto.Blake3Hash([]byte("cosi local"))
	remoteID := crypto.Blake3Hash([]byte("cosi remote"))
	node := &Node{IdForNetwork: localID}
	newSnapshot := func(nodeID crypto.Hash) *common.Snapshot {
		return &common.Snapshot{Version: common.SnapshotVersionCommonEncoding, NodeId: nodeID}
	}
	self := &Chain{
		node:            node,
		ChainId:         localID,
		CosiAggregators: make(map[crypto.Hash]*CosiAggregator),
		CosiVerifiers:   make(map[crypto.Hash]*CosiVerifier),
	}
	external := &Chain{
		node:            node,
		ChainId:         remoteID,
		CosiAggregators: make(map[crypto.Hash]*CosiAggregator),
		CosiVerifiers:   make(map[crypto.Hash]*CosiVerifier),
	}

	tests := []struct {
		name  string
		chain *Chain
		msg   *CosiAction
		want  string
	}{
		{
			name:  "self action on external chain",
			chain: external,
			msg:   &CosiAction{Action: CosiActionSelfEmpty, PeerId: remoteID, Snapshot: newSnapshot(remoteID)},
			want:  "self action announcement chain",
		},
		{
			name:  "self action from remote peer",
			chain: self,
			msg:   &CosiAction{Action: CosiActionSelfEmpty, PeerId: remoteID, Snapshot: newSnapshot(localID)},
			want:  "self action announcement peer",
		},
		{
			name:  "nonempty self action",
			chain: self,
			msg: &CosiAction{Action: CosiActionSelfEmpty, PeerId: localID, Snapshot: &common.Snapshot{
				Version:   common.SnapshotVersionCommonEncoding,
				NodeId:    localID,
				Timestamp: 1,
			}},
			want: "only empty snapshot",
		},
		{
			name:  "self aggregation on external chain",
			chain: external,
			msg:   &CosiAction{Action: CosiActionSelfCommitment, PeerId: localID, Snapshot: newSnapshot(remoteID)},
			want:  "self action aggregation chain",
		},
		{
			name:  "self aggregation from self",
			chain: self,
			msg:   &CosiAction{Action: CosiActionSelfResponse, PeerId: localID, Snapshot: newSnapshot(localID)},
			want:  "self action aggregation peer",
		},
		{
			name:  "external announcement on self chain",
			chain: self,
			msg:   &CosiAction{Action: CosiActionExternalAnnouncement, PeerId: localID, Snapshot: newSnapshot(localID)},
			want:  "external action announcement chain",
		},
		{
			name:  "external announcement from wrong peer",
			chain: external,
			msg:   &CosiAction{Action: CosiActionExternalAnnouncement, PeerId: localID, Snapshot: newSnapshot(remoteID)},
			want:  "external action announcement peer",
		},
		{
			name:  "external announcement without timestamp",
			chain: external,
			msg:   &CosiAction{Action: CosiActionExternalAnnouncement, PeerId: remoteID, Snapshot: newSnapshot(remoteID)},
			want:  "only empty snapshot with timestamp",
		},
		{
			name:  "full challenge without challenge",
			chain: external,
			msg: &CosiAction{Action: CosiActionExternalFullChallenge, PeerId: remoteID, Snapshot: &common.Snapshot{
				Version:   common.SnapshotVersionCommonEncoding,
				NodeId:    remoteID,
				Timestamp: 1,
			}},
			want: "timestamp and challenge",
		},
		{
			name:  "external challenge on self chain",
			chain: self,
			msg:   &CosiAction{Action: CosiActionExternalChallenge, PeerId: localID, Snapshot: newSnapshot(localID)},
			want:  "external action challenge chain",
		},
		{
			name:  "external challenge from wrong peer",
			chain: external,
			msg:   &CosiAction{Action: CosiActionExternalChallenge, PeerId: localID, Snapshot: newSnapshot(remoteID)},
			want:  "external action challenge peer",
		},
		{
			name:  "missing snapshot",
			chain: self,
			msg:   &CosiAction{Action: CosiActionSelfCommitment, PeerId: remoteID},
			want:  "no snapshot",
		},
		{
			name:  "invalid snapshot version",
			chain: self,
			msg:   &CosiAction{Action: CosiActionSelfCommitment, PeerId: remoteID, Snapshot: &common.Snapshot{NodeId: localID}},
			want:  "invalid snapshot version",
		},
		{
			name:  "invalid snapshot node",
			chain: self,
			msg:   &CosiAction{Action: CosiActionSelfCommitment, PeerId: remoteID, Snapshot: newSnapshot(remoteID)},
			want:  "invalid snapshot node id",
		},
		{
			name:  "missing chain state",
			chain: external,
			msg: &CosiAction{Action: CosiActionExternalAnnouncement, PeerId: remoteID, Snapshot: &common.Snapshot{
				Version:   common.SnapshotVersionCommonEncoding,
				NodeId:    remoteID,
				Timestamp: 1,
			}},
			want: "state empty",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.ErrorContains(t, test.chain.checkActionSanity(test.msg), test.want)
		})
	}

	require.False(t, shouldRequeueSelfAnnouncement(nil))
	require.False(t, shouldRequeueSelfAnnouncement(errors.New("unrelated")))
	require.True(t, shouldRequeueSelfAnnouncement(errors.New("chain not broadcasted")))
	require.True(t, shouldRequeueSelfAnnouncement(errors.New("node is slow in catching up")))
	require.True(t, shouldRequeueSelfAnnouncement(errors.New("transaction finalized in snapshot")))

	regular := common.NewTransactionV5(common.XINAssetId).AsVersioned()
	require.False(t, checkNodeAccept(map[crypto.Hash]*common.VersionedTransaction{regular.PayloadHash(): regular}))
	accept := common.NewTransactionV5(common.XINAssetId)
	accept.AddOutputWithType(common.OutputTypeNodeAccept, nil, common.Script{}, common.NewInteger(1), nil)
	accepted := accept.AsVersioned()
	require.True(t, checkNodeAccept(map[crypto.Hash]*common.VersionedTransaction{accepted.PayloadHash(): accepted}))
	require.Panics(t, func() {
		checkNodeAccept(map[crypto.Hash]*common.VersionedTransaction{
			accepted.PayloadHash(): accepted,
			regular.PayloadHash():  regular,
		})
	})
}

func TestNodeStateAndQueueHelpers(t *testing.T) {
	a := crypto.Blake3Hash([]byte("node a"))
	b := crypto.Blake3Hash([]byte("node b"))
	c := crypto.Blake3Hash([]byte("node c"))
	node := &Node{
		allNodesSortedWithState: []*CNode{
			{IdForNetwork: a, Timestamp: 10, State: common.NodeStateAccepted},
			{IdForNetwork: b, Timestamp: 20, State: common.NodeStatePledging},
			{IdForNetwork: a, Timestamp: 30, State: common.NodeStateRemoved},
			{IdForNetwork: c, Timestamp: 40, State: common.NodeStateAccepted},
		},
		genesisNodesMap: map[crypto.Hash]bool{a: true},
	}

	allAtTwentyFive := node.nodeSequenceWithoutState(25, false)
	require.Len(t, allAtTwentyFive, 2)
	require.Equal(t, a, allAtTwentyFive[0].IdForNetwork)
	require.Equal(t, b, allAtTwentyFive[1].IdForNetwork)
	require.Equal(t, 0, allAtTwentyFive[0].ConsensusIndex)
	require.Equal(t, 1, allAtTwentyFive[1].ConsensusIndex)
	require.Len(t, node.nodeSequenceWithoutState(25, true), 1)

	node.nodeStateSequences = node.buildNodeStateSequences(node.allNodesSortedWithState, false)
	node.acceptedNodeStateSequences = node.buildNodeStateSequences(node.allNodesSortedWithState, true)
	require.Nil(t, node.NodesListWithoutState(10, false))
	require.Len(t, node.NodesListWithoutState(25, false), 2)
	require.Len(t, node.NodesListWithoutState(45, true), 1)
	require.Equal(t, b, node.PledgingNode(25).IdForNetwork)
	require.Nil(t, node.PledgingNode(45))
	require.Equal(t, b, node.GetAcceptedOrPledgingNode(b).IdForNetwork)
	require.Nil(t, node.GetAcceptedOrPledgingNode(a))
	require.Equal(t, a, node.GetRemovedOrCancelledNode(a, 45).IdForNetwork)
	require.Nil(t, node.GetRemovedOrCancelledNode(b, 45))

	require.False(t, node.ConsensusReady(&CNode{IdForNetwork: a, State: common.NodeStatePledging}, 100))
	require.True(t, node.ConsensusReady(&CNode{IdForNetwork: a, State: common.NodeStateAccepted}, 100))
	require.False(t, node.ConsensusReady(&CNode{IdForNetwork: b, State: common.NodeStateAccepted, Timestamp: 100}, 101))
	require.True(t, node.ConsensusReady(&CNode{IdForNetwork: b, State: common.NodeStateAccepted}, uint64(config.KernelNodeAcceptPeriodMinimum)+1))

	consensusNodes := make([]*CNode, config.KernelMinimumNodesCount)
	genesis := make(map[crypto.Hash]bool)
	for i := range consensusNodes {
		id := crypto.Blake3Hash([]byte{byte(i + 1)})
		consensusNodes[i] = &CNode{IdForNetwork: id, State: common.NodeStateAccepted}
		genesis[id] = true
	}
	thresholdNode := &Node{
		nodeStateSequences: []*NodeStateSequence{{Timestamp: 1, NodesWithoutState: consensusNodes}},
		genesisNodesMap:    genesis,
	}
	require.Equal(t, config.KernelMinimumNodesCount*2/3+1, thresholdNode.ConsensusThreshold(2, true))
	thresholdNode.nodeStateSequences[0].NodesWithoutState = consensusNodes[:config.KernelMinimumNodesCount-1]
	require.Equal(t, 1000, thresholdNode.ConsensusThreshold(2, true))
	pledging := &CNode{
		IdForNetwork: crypto.Blake3Hash([]byte("threshold pledging")),
		State:        common.NodeStatePledging,
	}
	thresholdNode.nodeStateSequences[0].NodesWithoutState = append(consensusNodes[:config.KernelMinimumNodesCount-1], pledging)
	pledgingTimestamp := uint64(config.KernelNodeAcceptPeriodMinimum) + config.SnapshotReferenceThreshold*config.SnapshotRoundGap*3 + 1
	require.Equal(t, config.KernelMinimumNodesCount*2/3+1, thresholdNode.ConsensusThreshold(pledgingTimestamp, false))
	require.Equal(t, 1000, thresholdNode.ConsensusThreshold(pledgingTimestamp, true))
	require.Equal(t, uint8(common.SnapshotVersionCommonEncoding), node.SnapshotVersion())
	require.Equal(t, common.XINAssetId, node.NewTransaction(common.XINAssetId).Asset)

	t.Run("synchronized map returns a snapshot", func(t *testing.T) {
		points := &syncMap{mutex: new(sync.RWMutex), m: make(map[crypto.Hash]*p2p.SyncPoint)}
		point := &p2p.SyncPoint{NodeId: a}
		points.Set(a, point)
		copy := points.Map()
		require.Same(t, point, copy[a])
		delete(copy, a)
		require.Contains(t, points.Map(), a)
	})

	t.Run("queue selection", func(t *testing.T) {
		all := []*CNode{{IdForNetwork: a}, {IdForNetwork: b}}
		now := time.Unix(0, 0)
		require.Equal(t, []crypto.Hash{a}, node.findRandomHeadNodeWithPossibleTail(all, nil, nil, now))
		require.Equal(t, []crypto.Hash{a}, node.findRandomHeadNodeWithPossibleTail(all, []*CNode{{IdForNetwork: b}}, map[crypto.Hash]bool{a: true}, now))
		require.Equal(t, []crypto.Hash{a, b}, node.findRandomHeadNodeWithPossibleTail(all, []*CNode{{IdForNetwork: b}}, map[crypto.Hash]bool{b: true}, now))

		require.False(t, node.canBatchSelfTransactions())
		node.chain = &Chain{State: &ChainState{CacheRound: &CacheRound{}}}
		require.False(t, node.canBatchSelfTransactions())
		node.chain.State.CacheRound.Number = 1
		require.True(t, node.canBatchSelfTransactions())

		nowNanos := clock.NowUnixNano()
		node.chains = &chainsMap{m: map[crypto.Hash]*Chain{
			a: {State: &ChainState{FinalRound: &FinalRound{Start: nowNanos}}},
			b: {State: &ChainState{FinalRound: &FinalRound{Start: 1}}},
			c: {},
		}}
		node.chain.node = node
		leading, filter := node.filterLeadingNodes([]*CNode{{IdForNetwork: a}, {IdForNetwork: b}, {IdForNetwork: c}})
		require.Equal(t, []*CNode{{IdForNetwork: a}}, leading)
		require.Equal(t, map[crypto.Hash]bool{a: true}, filter)
	})

	t.Run("node completion signal", func(t *testing.T) {
		waiting := &Node{done: make(chan struct{})}
		require.False(t, waiting.waitOrDone(time.Nanosecond))
		close(waiting.done)
		require.True(t, waiting.waitOrDone(time.Hour))
	})
}
