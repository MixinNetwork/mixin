package kernel

import (
	"bytes"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/stretchr/testify/require"
)

func TestCosiResponseBeforeChallenge(t *testing.T) {
	var nodes []*CNode
	var keys []crypto.Key
	var publics []*crypto.Key
	genesis := make(map[crypto.Hash]bool)
	for i := range config.KernelMinimumNodesCount {
		key := crypto.NewKeyFromSeed(bytes.Repeat([]byte{byte(100 + i)}, 64))
		public := key.Public()
		id := crypto.Blake3Hash(public[:])
		keys = append(keys, key)
		publics = append(publics, &public)
		nodes = append(nodes, &CNode{IdForNetwork: id, ConsensusIndex: i, State: common.NodeStateAccepted, Signer: common.Address{PublicSpendKey: public}})
		genesis[id] = true
	}
	snapshot := &common.Snapshot{
		Version: common.SnapshotVersionCommonEncoding, NodeId: nodes[0].IdForNetwork,
		Timestamp: clock.NowUnixNano(), Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("early response"))},
	}
	snapshot.Hash = snapshot.PayloadHash()
	commitments := make(map[int]*crypto.CosiCommitment)
	nonces := make([]*crypto.CosiNonce, 5)
	for i := range nonces {
		nonces[i] = crypto.CosiCommitNonce(crypto.RandReader())
		commitments[i] = nonces[i].Public()
	}
	aggregator := &CosiAggregator{
		Snapshot: snapshot, Commitments: map[int]*crypto.CosiCommitment{0: commitments[0]},
		Responses: make(map[int]*[32]byte),
	}
	chain := &Chain{
		node:               &Node{genesisNodesMap: genesis, nodeStateSequences: []*NodeStateSequence{{NodesWithoutState: nodes}}},
		CosiAggregators:    map[crypto.Hash]*CosiAggregator{snapshot.Hash: aggregator},
		CosiCommunicatedAt: make(map[crypto.Hash]time.Time),
	}
	action := &CosiAction{
		PeerId: nodes[1].IdForNetwork, SnapshotHash: snapshot.Hash, Response: &[32]byte{},
		data: &CosiChainData{PN: nodes[1]},
	}
	require.NotPanics(t, func() { require.NoError(t, chain.cosiHandleResponse(action)) })
	require.Empty(t, aggregator.Responses)

	// Ignoring a premature response must not prevent the peer's valid response
	// after the commitment threshold has produced a challenge.
	var err error
	snapshot.Signature, err = crypto.CosiAggregateCommitment(commitments, publics, snapshot.Hash)
	require.NoError(t, err)
	aggregator.Commitments = commitments
	aggregator.Responses[0], err = nonces[0].Response(snapshot.Signature, &keys[0], publics, snapshot.Hash)
	require.NoError(t, err)
	action.Response, err = nonces[1].Response(snapshot.Signature, &keys[1], publics, snapshot.Hash)
	require.NoError(t, err)
	require.NoError(t, chain.cosiHandleResponse(action))
	require.Same(t, action.Response, aggregator.Responses[1])
}
