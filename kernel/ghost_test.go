package kernel

import (
	"fmt"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestGhostSeedReferenceEnabled(t *testing.T) {
	require := require.New(t)
	node := setupTestNode(require, t.TempDir())
	require.Equal(config.KernelNetworkId, node.networkId.String())

	require.False(node.ghostSeedReferenceEnabled(mainnetGhostSeedReferenceForkAt - 1))
	require.True(node.ghostSeedReferenceEnabled(mainnetGhostSeedReferenceForkAt))
	require.True(node.ghostSeedReferenceEnabled(mainnetGhostSeedReferenceForkAt + 1))

	// non mainnet networks always use the reference seed scheme
	node.networkId = crypto.Blake3Hash([]byte("ghost-seed-testnet"))
	require.True(node.ghostSeedReferenceEnabled(0))
}

func TestProtocolGhostSeed(t *testing.T) {
	require := require.New(t)

	anchor1 := crypto.Blake3Hash([]byte("anchor one"))
	anchor2 := crypto.Blake3Hash([]byte("anchor two"))

	// legacy scheme stays bit compatible with the old derivation
	in := "payeeNODEREMOVEsigner"
	si := crypto.Blake3Hash([]byte(in))
	legacy := append(si[:], si[:]...)
	require.Equal(legacy, protocolGhostSeed(in, crypto.Hash{}))

	// reference scheme is deterministic and anchor dependent
	s1 := protocolGhostSeed(in, anchor1)
	require.Equal(s1, protocolGhostSeed(in, anchor1))
	require.NotEqual(s1, protocolGhostSeed(in, anchor2))
	require.NotEqual(legacy, s1)

	// the output ghost key is only computable with the anchor hash
	r := crypto.NewKeyFromSeed(s1)
	viewSeed := make([]byte, 64)
	for i := range viewSeed {
		viewSeed[i] = byte(i + 1)
	}
	spendSeed := make([]byte, 64)
	for i := range spendSeed {
		spendSeed[i] = byte(i + 2)
	}
	A := crypto.NewKeyFromSeed(viewSeed).Public()
	B := crypto.NewKeyFromSeed(spendSeed).Public()
	k1 := crypto.DeriveGhostPublicKey(&r, &A, &B, 0)
	r2 := crypto.NewKeyFromSeed(protocolGhostSeed(in, anchor1))
	k2 := crypto.DeriveGhostPublicKey(&r2, &A, &B, 0)
	require.Equal(k1.String(), k2.String())
}

func TestGhostSeedReference(t *testing.T) {
	require := require.New(t)
	node := setupTestNode(require, t.TempDir())

	// The configured test signer is not an initialized genesis chain. Point
	// the node at one of the loaded genesis chains before exercising code that
	// requires a finalized local round.
	snaps, err := node.persistStore.ReadSnapshotsSinceTopology(0, 1)
	require.Nil(err)
	require.Len(snaps, 1)
	node.IdForNetwork = snaps[0].NodeId
	node.chain = node.getChain(node.IdForNetwork)
	require.NotNil(node.chain)
	require.NotNil(node.chain.State)
	require.NotNil(node.chain.State.FinalRound)

	// legacy timestamps need no anchor
	anchor, err := node.ghostSeedReference(mainnetGhostSeedReferenceForkAt - 1)
	require.Nil(err)
	require.False(anchor.HasValue())

	anchor, err = node.ghostSeedReference(mainnetGhostSeedReferenceForkAt + 1)
	require.Nil(err)
	require.True(anchor.HasValue())

	finalized, err := node.persistStore.ReadSnapshotsForNodeRound(
		node.IdForNetwork,
		node.chain.State.FinalRound.Number,
	)
	require.Nil(err)
	require.NotEmpty(finalized)
	require.Equal(finalized[0].Transactions[0], anchor)

	// the anchor is finalized and readable, as validators require
	rtx, snap, err := node.persistStore.ReadTransaction(anchor)
	require.Nil(err)
	require.NotNil(rtx)
	require.NotEmpty(snap)
}

func TestGhostSeedReferenceFromTx(t *testing.T) {
	require := require.New(t)
	node := setupTestNode(require, t.TempDir())

	anchor := crypto.Blake3Hash([]byte("proposer anchor"))
	consensusRef := crypto.Blake3Hash([]byte("consensus ref"))

	// legacy timestamps ignore the references
	got, err := node.ghostSeedReferenceFromTx([]crypto.Hash{consensusRef}, mainnetGhostSeedReferenceForkAt-1)
	require.Nil(err)
	require.False(got.HasValue())

	got, err = node.ghostSeedReferenceFromTx([]crypto.Hash{consensusRef, anchor}, mainnetGhostSeedReferenceForkAt+1)
	require.Nil(err)
	require.Equal(anchor.String(), got.String())

	_, err = node.ghostSeedReferenceFromTx([]crypto.Hash{consensusRef}, mainnetGhostSeedReferenceForkAt+1)
	require.ErrorContains(err, "invalid ghost seed references count")

	_, err = node.ghostSeedReferenceFromTx([]crypto.Hash{consensusRef, anchor, crypto.Blake3Hash([]byte("extra"))}, mainnetGhostSeedReferenceForkAt+1)
	require.ErrorContains(err, "invalid ghost seed references count")

	_, err = node.ghostSeedReferenceFromTx([]crypto.Hash{consensusRef, {}}, mainnetGhostSeedReferenceForkAt+1)
	require.ErrorContains(err, "invalid empty ghost seed reference")
}

func TestNodeRemoveTransactionGhostSeedReference(t *testing.T) {
	require := require.New(t)
	node := setupTestNode(require, t.TempDir())

	// same epoch hour as the legacy remove test, after the fork: adding whole
	// days preserves the epoch hour of 2021-03-10T17:00:00Z
	now, err := time.Parse(time.RFC3339, "2021-03-10T17:00:00Z")
	require.Nil(err)
	days := (mainnetGhostSeedReferenceForkAt-uint64(now.UnixNano()))/uint64(24*time.Hour) + 1
	postFork := uint64(now.UnixNano()) + days*uint64(24*time.Hour)
	require.Greater(postFork, mainnetGhostSeedReferenceForkAt)

	anchor1 := crypto.Blake3Hash([]byte("reference one"))
	anchor2 := crypto.Blake3Hash([]byte("reference two"))

	tx1, err := node.buildNodeRemoveTransaction(node.IdForNetwork, postFork, anchor1, nil)
	require.Nil(err)
	require.NotNil(tx1)

	// the references carry the consensus reference followed by the anchor,
	// and the extra is untouched
	require.Len(tx1.References, 2)
	consensusSnap, _ := node.ReadLastConsensusSnapshotWithHack()
	require.Equal(consensusSnap.Transactions[0].String(), tx1.References[0].String())
	require.Equal(anchor1.String(), tx1.References[1].String())
	accept, _, err := node.persistStore.ReadTransaction(tx1.Inputs[0].Hash)
	require.Nil(err)
	require.Equal(accept.Extra, []byte(tx1.Extra))

	// validators rebuild the same transaction from the referenced anchor
	got, err := node.ghostSeedReferenceFromTx(tx1.References, postFork)
	require.Nil(err)
	require.Equal(anchor1.String(), got.String())
	rebuilt, err := node.buildNodeRemoveTransaction(node.IdForNetwork, postFork, got, tx1)
	require.Nil(err)
	require.Equal(tx1.PayloadHash().String(), rebuilt.PayloadHash().String())

	// a different anchor yields a different ghost key and payload
	tx2, err := node.buildNodeRemoveTransaction(node.IdForNetwork, postFork, anchor2, nil)
	require.Nil(err)
	require.NotEqual(tx1.PayloadHash().String(), tx2.PayloadHash().String())
	require.NotEqual(tx1.Outputs[0].Keys[0].String(), tx2.Outputs[0].Keys[0].String())

	// the new ghost key differs from the legacy publicly derivable one
	candi, err := node.checkRemovePossibility(node.IdForNetwork, postFork, nil)
	require.Nil(err)
	in := fmt.Sprintf("NODEREMOVE%s", candi.Signer.String())
	si := crypto.Blake3Hash([]byte(candi.Payee.String() + in))
	r := crypto.NewKeyFromSeed(append(si[:], si[:]...))
	legacyKey := crypto.DeriveGhostPublicKey(&r, &candi.Payee.PublicViewKey, &candi.Payee.PublicSpendKey, 0)
	require.NotEqual(legacyKey.String(), tx1.Outputs[0].Keys[0].String())
}
