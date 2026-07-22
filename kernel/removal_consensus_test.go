package kernel

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/stretchr/testify/require"
)

const removalConsensusNodeCount = config.KernelMinimumNodesCount + 2

type removalConsensusFixture struct {
	epoch       uint64
	accepted    []*CNode
	privateKeys map[crypto.Hash]*crypto.Key
	genesis     map[crypto.Hash]bool
	networkID   crypto.Hash
}

func newRemovalConsensusFixture(t *testing.T) *removalConsensusFixture {
	t.Helper()

	// Keep the fork at hour 13 relative to the synthetic epoch so the tests
	// remain valid if the activation date is deliberately moved later.
	epoch := mainnetConsensusNodeRemovalSignerSetForkAt - 100*OneDay -
		uint64(config.KernelNodeAcceptTimeBegin)*uint64(time.Hour)
	accepted := make([]*CNode, removalConsensusNodeCount)
	privateKeys := make(map[crypto.Hash]*crypto.Key, removalConsensusNodeCount)
	genesis := make(map[crypto.Hash]bool, removalConsensusNodeCount)
	for i := range removalConsensusNodeCount {
		seed := crypto.Blake3Hash(fmt.Appendf(nil, "removal signer %d", i))
		private := crypto.NewKeyFromSeed(append(seed[:], seed[:]...))
		public := private.Public()
		id := crypto.Blake3Hash(fmt.Appendf(nil, "removal node %d", i))
		accepted[i] = &CNode{
			IdForNetwork: id,
			Signer:       common.Address{PublicSpendKey: public},
			Timestamp:    epoch + uint64(i),
			State:        common.NodeStateAccepted,
		}
		key := private
		privateKeys[id] = &key
		genesis[id] = true
	}
	networkID, err := crypto.HashFromString(config.KernelNetworkId)
	require.NoError(t, err)
	return &removalConsensusFixture{
		epoch:       epoch,
		accepted:    accepted,
		privateKeys: privateKeys,
		genesis:     genesis,
		networkID:   networkID,
	}
}

func (f *removalConsensusFixture) newNode(t *testing.T, states []*CNode) *Node {
	t.Helper()
	node := &Node{
		Epoch:                   f.epoch,
		networkId:               f.networkID,
		allNodesSortedWithState: states,
		genesisNodesMap:         f.genesis,
	}
	node.nodeStateSequences = node.buildNodeStateSequences(states, false)
	node.acceptedNodeStateSequences = node.buildNodeStateSequences(states, true)
	cache, err := ristretto.NewCache(&ristretto.Config[[]byte, any]{
		NumCounters: 1e3,
		MaxCost:     1 << 20,
		BufferItems: 64,
	})
	require.NoError(t, err)
	t.Cleanup(cache.Close)
	node.cacheStore = cache
	return node
}

func (f *removalConsensusFixture) nodeAfterRemoval(t *testing.T, removalTime uint64) *Node {
	t.Helper()
	removed := *f.accepted[0]
	removed.Timestamp = removalTime
	removed.State = common.NodeStateRemoved
	states := append(slices.Clone(f.accepted), &removed)
	return f.newNode(t, states)
}

func (f *removalConsensusFixture) chain(node *Node) *Chain {
	return &Chain{node: node, ChainId: f.accepted[1].IdForNetwork}
}

func (f *removalConsensusFixture) signSnapshot(t *testing.T, chain *Chain, timestamp uint64, label string) (*common.Snapshot, []crypto.Hash) {
	t.Helper()
	ids, publics := chain.ConsensusKeys(1, timestamp)
	threshold := chain.node.ConsensusThreshold(timestamp, true)
	require.LessOrEqual(t, threshold, len(ids))

	snapshot := &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       chain.ChainId,
		RoundNumber:  1,
		Timestamp:    timestamp,
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte(label))},
	}
	snapshot.Hash = snapshot.PayloadHash()

	nonces := make(map[int]*crypto.CosiNonce, threshold)
	commitments := make(map[int]*crypto.Key, threshold)
	for i := range threshold {
		nonce := crypto.CosiCommitNonce(crypto.RandReader())
		commitment := nonce.Public()
		nonces[i] = nonce
		commitments[i] = &commitment
	}
	signature, err := crypto.CosiAggregateCommitment(commitments)
	require.NoError(t, err)
	responses := make(map[int]*[32]byte, threshold)
	for i := range threshold {
		response, err := nonces[i].Response(
			signature,
			f.privateKeys[ids[i]],
			publics,
			snapshot.Hash,
		)
		require.NoError(t, err)
		responses[i] = response
	}
	require.NoError(t, signature.AggregateResponse(publics, responses, snapshot.Hash, true))
	snapshot.Signature = signature
	return snapshot, ids[:threshold]
}

func TestLegacyConsensusCertificateAcrossNodeRemoval(t *testing.T) {
	f := newRemovalConsensusFixture(t)
	operationStart := mainnetConsensusNodeRemovalSignerSetForkAt - OneDay
	removalTime := operationStart + uint64(30*time.Second)
	snapshotTime := removalTime + uint64(time.Second)

	signingNode := f.newNode(t, slices.Clone(f.accepted))
	verifyingNode := f.nodeAfterRemoval(t, removalTime)
	require.False(t, signingNode.usePredictiveNodeRemovalSignerSet(snapshotTime))

	signingChain := f.chain(signingNode)
	verifyingChain := f.chain(verifyingNode)
	signingIDs, _ := signingChain.ConsensusKeys(1, snapshotTime)
	verifyingIDs, _ := verifyingChain.ConsensusKeys(1, snapshotTime)
	require.Len(t, signingIDs, removalConsensusNodeCount)
	require.Len(t, verifyingIDs, removalConsensusNodeCount-1)
	require.NotEqual(t, signingIDs, verifyingIDs)
	require.Equal(t, removalConsensusNodeCount*2/3+1,
		signingNode.ConsensusThreshold(snapshotTime, true))

	snapshot, expectedSigners := f.signSnapshot(t, signingChain, snapshotTime, "legacy removal certificate")
	signers, finalized := verifyingChain.verifyFinalization(snapshot)
	require.True(t, finalized)
	require.Equal(t, expectedSigners, signers)
}

func TestRemovalConsensusSignerSetForkBoundary(t *testing.T) {
	f := newRemovalConsensusFixture(t)
	node := f.newNode(t, slices.Clone(f.accepted))
	chain := f.chain(node)
	before, at := mainnetConsensusNodeRemovalSignerSetForkAt-1,
		mainnetConsensusNodeRemovalSignerSetForkAt

	require.False(t, node.usePredictiveNodeRemovalSignerSet(before))
	require.True(t, node.usePredictiveNodeRemovalSignerSet(at))
	beforeIDs, _ := chain.ConsensusKeys(1, before)
	atIDs, _ := chain.ConsensusKeys(1, at)
	require.Len(t, beforeIDs, removalConsensusNodeCount)
	require.Len(t, atIDs, removalConsensusNodeCount-1)
	require.Contains(t, beforeIDs, f.accepted[0].IdForNetwork)
	require.NotContains(t, atIDs, f.accepted[0].IdForNetwork)
	require.Equal(t, removalConsensusNodeCount*2/3+1,
		node.ConsensusThreshold(before, true))
	require.Equal(t, (removalConsensusNodeCount-1)*2/3+1,
		node.ConsensusThreshold(at, true))
}

func TestConsensusConfigurationStableAcrossNodeRemoval(t *testing.T) {
	f := newRemovalConsensusFixture(t)
	operationStart := mainnetConsensusNodeRemovalSignerSetForkAt
	removalTime := operationStart + uint64(30*time.Second)
	snapshotTime := removalTime + uint64(time.Second)

	// The signer has not received the removal finalization yet, while the
	// verifier has. Both must derive the same signer vector for a concurrent
	// snapshot whose timestamp follows the removal snapshot.
	signingNode := f.newNode(t, slices.Clone(f.accepted))
	verifyingNode := f.nodeAfterRemoval(t, removalTime)
	require.Equal(t, f.accepted[0].IdForNetwork,
		signingNode.removingOrSlashingNodeAt(snapshotTime).IdForNetwork)
	require.Equal(t, f.accepted[0].IdForNetwork,
		verifyingNode.removingOrSlashingNodeAt(snapshotTime).IdForNetwork)

	signingChain := f.chain(signingNode)
	verifyingChain := f.chain(verifyingNode)
	signingIDs, signingPublics := signingChain.ConsensusKeys(1, snapshotTime)
	verifyingIDs, verifyingPublics := verifyingChain.ConsensusKeys(1, snapshotTime)

	require.Len(t, signingIDs, removalConsensusNodeCount-1)
	require.Equal(t, signingIDs, verifyingIDs)
	require.Equal(t, signingPublics, verifyingPublics)
	require.NotContains(t, signingIDs, f.accepted[0].IdForNetwork)
	for i, participant := range signingChain.consensusNodes(1, snapshotTime) {
		require.Equal(t, i, participant.ConsensusIndex)
	}
	require.Equal(t,
		signingChain.cosiAcceptedNodesListShuffle(1, snapshotTime),
		verifyingChain.cosiAcceptedNodesListShuffle(1, snapshotTime),
	)

	signingThreshold := signingNode.ConsensusThreshold(snapshotTime, true)
	verifyingThreshold := verifyingNode.ConsensusThreshold(snapshotTime, true)
	require.Equal(t, (removalConsensusNodeCount-1)*2/3+1, signingThreshold)
	require.Equal(t, signingThreshold, verifyingThreshold)

	snapshot, expectedSigners := f.signSnapshot(t, signingChain, snapshotTime, "predictive removal certificate")
	signers, finalized := verifyingChain.verifyFinalization(snapshot)
	require.True(t, finalized)
	require.Equal(t, expectedSigners, signers)
}

func TestRemovalConsensusOperationWindowBoundary(t *testing.T) {
	f := newRemovalConsensusFixture(t)
	operationStart := mainnetConsensusNodeRemovalSignerSetForkAt
	removalTime := operationStart + uint64(30*time.Second)
	lastInWindow := operationStart +
		uint64(config.KernelNodeAcceptTimeEnd-config.KernelNodeAcceptTimeBegin+1)*uint64(time.Hour) - 1
	firstAfterWindow := lastInWindow + 1

	unawareNode := f.newNode(t, slices.Clone(f.accepted))
	awareNode := f.nodeAfterRemoval(t, removalTime)
	unawareChain, awareChain := f.chain(unawareNode), f.chain(awareNode)

	require.NotNil(t, unawareNode.removingOrSlashingNodeAt(lastInWindow))
	require.Nil(t, unawareNode.removingOrSlashingNodeAt(firstAfterWindow))
	unawareInWindow, _ := unawareChain.ConsensusKeys(1, lastInWindow)
	awareInWindow, _ := awareChain.ConsensusKeys(1, lastInWindow)
	require.Equal(t, unawareInWindow, awareInWindow)
	require.Len(t, unawareInWindow, removalConsensusNodeCount-1)

	// This inequality documents the known post-window propagation race: an
	// unaware node includes the candidate again at 20:00, while a node that has
	// finalized the removal continues to exclude it through ledger state.
	unawareAfterWindow, _ := unawareChain.ConsensusKeys(1, firstAfterWindow)
	awareAfterWindow, _ := awareChain.ConsensusKeys(1, firstAfterWindow)
	require.NotEqual(t, unawareAfterWindow, awareAfterWindow)
	require.Len(t, unawareAfterWindow, removalConsensusNodeCount)
	require.Len(t, awareAfterWindow, removalConsensusNodeCount-1)
}

func TestRemovalConsensusCandidateProgressesNextDay(t *testing.T) {
	f := newRemovalConsensusFixture(t)
	operationStart := mainnetConsensusNodeRemovalSignerSetForkAt
	removalTime := operationStart + uint64(30*time.Second)
	nextOperationStart := operationStart + OneDay
	node := f.nodeAfterRemoval(t, removalTime)

	current := node.removingOrSlashingNodeAt(removalTime + uint64(time.Second))
	next := node.removingOrSlashingNodeAt(nextOperationStart)
	require.NotNil(t, current)
	require.NotNil(t, next)
	require.Equal(t, f.accepted[0].IdForNetwork, current.IdForNetwork)
	require.Equal(t, f.accepted[1].IdForNetwork, next.IdForNetwork)

	ids, _ := f.chain(node).ConsensusKeys(1, nextOperationStart)
	require.NotContains(t, ids, f.accepted[0].IdForNetwork)
	require.NotContains(t, ids, f.accepted[1].IdForNetwork)
	require.Len(t, ids, removalConsensusNodeCount-2)
}
