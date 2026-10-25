package kernel

import (
	"bytes"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/p2p"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/stretchr/testify/require"
)

type missingFinalizationTransactions struct{ storage.Store }

func (*missingFinalizationTransactions) ReadTransaction(crypto.Hash) (*common.VersionedTransaction, string, error) {
	return nil, "", nil
}

func (*missingFinalizationTransactions) CacheGetTransaction(crypto.Hash) (*common.VersionedTransaction, error) {
	return nil, nil
}

func TestFinalizationRetainsForwardingPeer(t *testing.T) {
	cache, err := ristretto.NewCache(&ristretto.Config[[]byte, any]{NumCounters: 1000, MaxCost: 1024 * 1024, BufferItems: 64})
	require.NoError(t, err)
	t.Cleanup(cache.Close)
	node := &Node{
		IdForNetwork:    crypto.Blake3Hash([]byte("local node")),
		Signer:          common.NewAddressFromSeed(bytes.Repeat([]byte{90}, 64)),
		cacheStore:      cache,
		persistStore:    &missingFinalizationTransactions{},
		genesisNodesMap: make(map[crypto.Hash]bool),
	}
	var nodes []*CNode
	var keys []crypto.Key
	var publics []*crypto.Key
	for i := range config.KernelMinimumNodesCount {
		key := crypto.NewKeyFromSeed(bytes.Repeat([]byte{byte(30 + i)}, 64))
		public := key.Public()
		id := crypto.Blake3Hash(public[:])
		keys = append(keys, key)
		publics = append(publics, &public)
		nodes = append(nodes, &CNode{IdForNetwork: id, State: common.NodeStateAccepted, Signer: common.Address{PublicSpendKey: public}})
		node.genesisNodesMap[id] = true
	}
	node.nodeStateSequences = []*NodeStateSequence{{NodesWithoutState: nodes}}
	node.Peer = p2p.NewPeer(node, node.IdForNetwork, "test", false)
	owner := nodes[0].IdForNetwork
	sender := nodes[1].IdForNetwork
	chain := &Chain{node: node, ChainId: owner, finalActionsRing: make(ActionBuffer, 1)}
	node.chains = &chainsMap{m: map[crypto.Hash]*Chain{owner: chain}}
	snapshot := &common.Snapshot{
		Version:      common.SnapshotVersionCommonEncoding,
		NodeId:       owner,
		RoundNumber:  1,
		Timestamp:    clock.NowUnixNano(),
		References:   &common.RoundLink{},
		Transactions: []crypto.Hash{crypto.Blake3Hash([]byte("missing payload"))},
	}
	commitments := make(map[int]*crypto.CosiCommitment)
	nonces := make([]*crypto.CosiNonce, len(keys))
	for i := range keys {
		nonces[i] = crypto.CosiCommitNonce(crypto.RandReader())
		commitments[i] = nonces[i].Public()
	}
	message := snapshot.PayloadHash()
	signature, err := crypto.CosiAggregateCommitment(commitments, publics, message)
	require.NoError(t, err)
	responses := make(map[int]*[32]byte)
	for i := range keys {
		responses[i], err = nonces[i].Response(signature, &keys[i], publics, message)
		require.NoError(t, err)
	}
	require.NoError(t, signature.AggregateResponse(publics, responses, message, true))
	snapshot.Signature = signature
	require.NoError(t, node.VerifyAndQueueAppendSnapshotFinalization(sender, snapshot))
	select {
	case action := <-chain.finalActionsRing:
		require.Equal(t, sender, action.PeerId)
		require.Same(t, snapshot, action.Snapshot)
	default:
		t.Fatal("valid finalization was not queued")
	}
}
