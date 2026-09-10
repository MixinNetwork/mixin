package kernel

import (
	"fmt"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/stretchr/testify/require"
)

// TestUniversalMintTransactionGhostSeedReference verifies that under the
// reference seed scheme the mint outputs mix a recent transaction hash into
// every ghost key seed, that the hash is recorded in the transaction
// references, and that validators rebuild the identical transaction from it.
func TestUniversalMintTransactionGhostSeedReference(t *testing.T) {
	require := require.New(t)
	logger.SetLevel(0)

	root := t.TempDir()

	internal.ToggleMockRunAggregators(true)
	node := setupTestNode(require, root)
	require.NotNil(node)

	snaps, err := node.persistStore.ReadSnapshotsSinceTopology(0, 100)
	require.Nil(err)
	require.Len(snaps, 28)
	node.IdForNetwork = snaps[0].NodeId

	addr := "XINYneY2gomSHxkYF62pxbNdwcdhcayxJRAeyUanJR611q5NWg4QebfFhEF3Me8qCHR8g8tD6QHPHD8naZnnn3GdRrhhiuxi"
	custodian, _ := common.NewAddressFromString(addr)

	amount := common.NewIntegerFromString("89.87671232")
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddUniversalMintInput(uint64(1706), amount)
	tx.AddScriptOutput([]*common.Address{&custodian}, common.NewThresholdScript(1), amount, make([]byte, 64))
	versioned := tx.AsVersioned()
	err = versioned.LockInputs(node.persistStore, false)
	require.Nil(err)
	err = node.persistStore.WriteTransaction(versioned)
	require.Nil(err)

	legacy := time.Date(2023, time.Month(10), 31, 8, 0, 0, 0, time.UTC)
	clock.MockDiff(legacy.Sub(clock.Now()))
	snap := &common.Snapshot{
		Version:     common.SnapshotVersionCommonEncoding,
		NodeId:      node.IdForNetwork,
		RoundNumber: 1,
		Timestamp:   uint64(legacy.UnixNano()),
		Signature:   &crypto.CosiSignature{Mask: 1},
	}
	snap.AddTransaction(versioned.PayloadHash())
	cache, err := loadHeadRoundForNode(node.persistStore, node.IdForNetwork)
	require.Nil(err)
	require.NotNil(cache)
	snap.References = &common.RoundLink{
		Self:     cache.References.Self,
		External: cache.References.External,
	}
	snap.Hash = snap.PayloadHash()
	node.TopoWrite(snap, []crypto.Hash{snap.NodeId})

	signers := node.genesisNodes
	for _, tr := range []struct {
		diff  time.Duration
		round uint64
	}{{
		diff:  time.Hour,
		round: 0,
	}, {
		diff:  time.Hour * 23,
		round: 1,
	}} {
		clock.MockDiff(tr.diff)
		timestamp := clock.NowUnixNano()
		for range 2 {
			snapshots := testBuildMintSnapshots(signers, tr.round, timestamp)
			err = node.persistStore.WriteRoundWork(node.IdForNetwork, tr.round, snapshots, true)
			require.Nil(err)
			for j := 1; j < 2*len(signers)/3+1; j++ {
				err = node.persistStore.WriteRoundWork(signers[j], tr.round, snapshots, true)
				require.Nil(err)
			}
		}
		batch := (timestamp - node.Epoch) / (24 * uint64(time.Hour))
		for i, id := range signers {
			if i == len(signers)*2/3+1 {
				break
			}
			err = node.persistStore.WriteRoundSpaceAndState(&common.RoundSpace{
				NodeId:   id,
				Batch:    batch,
				Round:    tr.round,
				Duration: 0,
			})
			require.Nil(err)
		}
	}

	// jump past the ghost seed reference fork, keeping the epoch hour aligned
	// so the jump lands on whole days
	now := clock.NowUnixNano()
	days := (mainnetGhostSeedReferenceForkAt-now)/uint64(24*time.Hour) + 1
	clock.MockDiff(time.Duration(days * uint64(24*time.Hour)))
	timestamp := clock.NowUnixNano()
	require.Greater(timestamp, mainnetGhostSeedReferenceForkAt)

	// write the works and spaces of the day the mint build requires
	for _, round := range []uint64{2, 3} {
		ts := timestamp
		if round == 2 {
			ts = timestamp - uint64(24*time.Hour)
		}
		snapshots := testBuildMintSnapshots(signers, round, ts)
		err = node.persistStore.WriteRoundWork(node.IdForNetwork, round, snapshots, true)
		require.Nil(err)
		for j := 1; j < 2*len(signers)/3+1; j++ {
			err = node.persistStore.WriteRoundWork(signers[j], round, snapshots, true)
			require.Nil(err)
		}
	}
	batch := (timestamp - node.Epoch) / (24 * uint64(time.Hour))
	for i, id := range signers {
		if i == len(signers)*2/3+1 {
			break
		}
		err = node.persistStore.WriteRoundSpaceAndState(&common.RoundSpace{
			NodeId:   id,
			Batch:    batch,
			Round:    3,
			Duration: 0,
		})
		require.Nil(err)
	}

	cur, err := node.persistStore.ReadCustodian(timestamp)
	require.Nil(err)
	require.NotNil(cur)

	anchor1 := crypto.Blake3Hash([]byte("mint proposer anchor one"))
	anchor2 := crypto.Blake3Hash([]byte("mint proposer anchor two"))

	versioned = node.buildUniversalMintTransaction(cur, timestamp, anchor1, false)
	require.NotNil(versioned)
	require.Empty(versioned.Extra)

	// the references carry the consensus reference followed by the anchor
	require.Len(versioned.References, 2)
	consensusSnap, _ := node.ReadLastConsensusSnapshotWithHack()
	require.Equal(consensusSnap.Transactions[0].String(), versioned.References[0].String())
	require.Equal(anchor1.String(), versioned.References[1].String())

	// validators extract the anchor from the references and rebuild the mint
	got, err := node.ghostSeedReferenceFromTx(versioned.References, timestamp)
	require.Nil(err)
	require.Equal(anchor1.String(), got.String())
	rebuilt := node.buildUniversalMintTransaction(cur, timestamp, got, true)
	require.NotNil(rebuilt)
	require.Equal(versioned.PayloadHash().String(), rebuilt.PayloadHash().String())

	// a different anchor changes every ghost key
	other := node.buildUniversalMintTransaction(cur, timestamp, anchor2, false)
	require.NotNil(other)
	require.NotEqual(versioned.PayloadHash().String(), other.PayloadHash().String())
	for i := range versioned.Outputs {
		require.NotEqual(versioned.Outputs[i].Keys[0].String(), other.Outputs[i].Keys[0].String())
	}

	// the keys differ from the legacy publicly derivable scheme
	accepted := node.NodesListWithoutState(timestamp, true)
	require.NotEmpty(accepted)
	first := accepted[0]
	si := crypto.Blake3Hash(fmt.Appendf(nil, "%sMINTKERNELNODE%d", first.Signer, batch))
	r := crypto.NewKeyFromSeed(append(si[:], si[:]...))
	legacyKey := crypto.DeriveGhostPublicKey(&r, &first.Payee.PublicViewKey, &first.Payee.PublicSpendKey, 0)
	require.NotEqual(legacyKey.String(), versioned.Outputs[0].Keys[0].String())

	// tampered or legacy references are rejected by the snapshot validator
	snap = &common.Snapshot{
		NodeId:    node.electSnapshotNode(common.TransactionTypeMint, timestamp),
		Timestamp: timestamp,
	}
	err = node.validateMintSnapshot(snap, versioned)
	require.Nil(err)

	bad := node.buildUniversalMintTransaction(cur, timestamp, anchor1, false)
	require.NotNil(bad)
	bad.References = bad.References[:1]
	err = node.validateMintSnapshot(snap, bad)
	require.ErrorContains(err, "invalid ghost seed references count")

	bad = node.buildUniversalMintTransaction(cur, timestamp, anchor1, false)
	require.NotNil(bad)
	bad.References[1] = crypto.Hash{}
	err = node.validateMintSnapshot(snap, bad)
	require.ErrorContains(err, "invalid empty ghost seed reference")
}
