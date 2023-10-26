package kernel

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/stretchr/testify/require"
)

func TestPledgeAmount(t *testing.T) {
	require := require.New(t)

	require.Equal(common.NewIntegerFromString("13439"), KernelNodePledgeAmount)
}

func TestPoolSize(t *testing.T) {
	require := require.New(t)

	require.Equal(common.NewInteger(500000), poolSizeUniversal(0))
	require.Equal(common.NewIntegerFromString("498630.13698640"), poolSizeUniversal(10))
	require.Equal(common.NewInteger(500000), poolSizeUniversal(0))
	require.Equal(common.NewIntegerFromString("450000"), poolSizeUniversal(365))
	require.Equal(common.NewIntegerFromString("449876.71232877"), poolSizeUniversal(366))
	require.Equal(common.NewIntegerFromString("307917.61644032"), poolSizeUniversal(1684))
	require.Equal(common.NewIntegerFromString("305850.45205696"), poolSizeUniversal(1707))
}

func TestUniversalMintTransaction(t *testing.T) {
	require := require.New(t)
	logger.SetLevel(0)

	root, err := os.MkdirTemp("", "mixin-mint-test")
	require.Nil(err)
	defer os.RemoveAll(root)

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
	require.Equal(amount.String(), node.lastMintDistribution().Amount.String())
	require.Equal(uint64(1706), node.lastMintDistribution().Batch)

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
	snap.AddSoleTransaction(versioned.PayloadHash())
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
		timestamp := uint64(clock.Now().UnixNano())
		for i := 0; i < 2; i++ {
			snapshots := testBuildMintSnapshots(signers, tr.round, timestamp)
			err = node.persistStore.WriteRoundWork(node.IdForNetwork, tr.round, snapshots)
			require.Nil(err)
			for j := 1; j < 2*len(signers)/3+1; j++ {
				err = node.persistStore.WriteRoundWork(signers[j], tr.round, snapshots)
				require.Nil(err)
			}

			day := uint32(snapshots[0].Timestamp / uint64(time.Hour*24))
			works, err := node.persistStore.ListNodeWorks(signers, day)
			require.Nil(err)
			require.Len(works, len(signers))
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

	timestamp := uint64(clock.Now().UnixNano())
	cur := &common.CustodianUpdateRequest{Custodian: &custodian}
	versioned = node.buildUniversalMintTransaction(cur, timestamp, false)
	require.NotNil(versioned)

	amount = common.NewIntegerFromString("89.87671232")
	mint := versioned.Inputs[0].Mint
	require.Equal(KernelNetworkLegacyEnding+1, mint.Batch)
	require.Equal("UNIVERSAL", mint.Group)
	require.Equal(amount.String(), mint.Amount.String())
	require.Len(versioned.Outputs, len(signers)+2)
	var kernel, safe, light common.Integer
	for i, o := range versioned.Outputs {
		if i == len(signers) {
			safe = o.Amount
			require.Equal("fffe01", o.Script.String())
		} else if i == len(signers)+1 {
			light = o.Amount
			require.Equal("fffe40", o.Script.String())
		} else {
			kernel = kernel.Add(o.Amount)
			require.Equal("fffe01", o.Script.String())
		}
	}
	require.Equal(common.NewIntegerFromString("44.93835595"), kernel)
	require.Equal(common.NewIntegerFromString("35.95068492"), safe)
	require.Equal(common.NewIntegerFromString("8.98767145"), light)
}

func TestMintWorks(t *testing.T) {
	require := require.New(t)

	root, err := os.MkdirTemp("", "mixin-mint-test")
	require.Nil(err)
	defer os.RemoveAll(root)

	internal.ToggleMockRunAggregators(true)

	node := setupTestNode(require, root)
	require.NotNil(node)

	offset, err := node.persistStore.ReadWorkOffset(node.IdForNetwork)
	require.Nil(err)
	require.Equal(uint64(0), offset)

	signers := append(node.genesisNodes, node.IdForNetwork)
	timestamp := uint64(clock.Now().UnixNano())
	leaders := len(signers)*2/3 + 1
	for i := 0; i < 2; i++ {
		snapshots := testBuildMintSnapshots(signers[1:], 0, timestamp)
		err = node.persistStore.WriteRoundWork(node.IdForNetwork, 0, snapshots)
		require.Nil(err)
		for j := 1; j < leaders; j++ {
			err = node.persistStore.WriteRoundWork(signers[j], 0, snapshots)
			require.Nil(err)
		}

		works, err := node.persistStore.ListNodeWorks(signers, uint32(snapshots[0].Timestamp/uint64(time.Hour*24)))
		require.Nil(err)
		require.Len(works, len(signers))
		for i, id := range signers {
			if i == 0 {
				require.Equal(uint64(0), works[id][0])
				require.Equal(uint64(0), works[id][1])
			} else if i < leaders {
				require.Equal(uint64(100), works[id][0])
				require.Equal(uint64(100*(leaders-1)), works[id][1])
			} else if i < len(node.genesisNodes) {
				require.Equal(uint64(0), works[id][0])
				require.Equal(uint64(100*leaders), works[id][1])
			} else {
				require.Equal(uint64(100), works[id][0])
				require.Equal(uint64(100*(leaders-1)), works[id][1])
			}
		}
		offset, err := node.persistStore.ReadWorkOffset(node.IdForNetwork)
		require.Nil(err)
		require.Equal(uint64(0), offset)
	}

	timestamp = uint64(clock.Now().UnixNano())
	snapshots := testBuildMintSnapshots(signers[1:], 1, timestamp)
	err = node.persistStore.WriteRoundWork(node.IdForNetwork, 1, snapshots[:98])
	require.Nil(err)

	works, err := node.persistStore.ListNodeWorks(signers, uint32(snapshots[0].Timestamp/uint64(time.Hour*24)))
	require.Nil(err)
	require.Len(works, len(signers))
	require.Equal(uint64(198), works[node.IdForNetwork][0])
	require.Equal(uint64(100*(leaders-1)), works[node.IdForNetwork][1])
	for i, id := range signers {
		if i == 0 {
			require.Equal(uint64(0), works[id][0])
			require.Equal(uint64(0), works[id][1])
		} else if i < leaders {
			require.Equal(uint64(100), works[id][0])
			require.Equal(uint64(100*(leaders-1)+98), works[id][1])
		} else if i < len(node.genesisNodes) {
			require.Equal(uint64(0), works[id][0])
			require.Equal(uint64(100*leaders+98), works[id][1])
		} else {
			require.Equal(uint64(198), works[id][0])
			require.Equal(uint64(100*(leaders-1)), works[id][1])
		}
	}
	offset, err = node.persistStore.ReadWorkOffset(node.IdForNetwork)
	require.Nil(err)
	require.Equal(uint64(1), offset)

	err = node.persistStore.WriteRoundWork(node.IdForNetwork, 1, snapshots)
	require.Nil(err)
	for i := 1; i < leaders; i++ {
		err = node.persistStore.WriteRoundWork(signers[i], 1, nil)
		require.Nil(err)
	}

	works, err = node.persistStore.ListNodeWorks(signers, uint32(snapshots[0].Timestamp/uint64(time.Hour*24)))
	require.Nil(err)
	require.Len(works, len(node.genesisNodes)+1)
	require.Equal(uint64(200), works[node.IdForNetwork][0])
	require.Equal(uint64(100*(leaders-1)), works[node.IdForNetwork][1])
	for i, id := range signers {
		if i == 0 { // 0
			require.Equal(uint64(0), works[id][0])
			require.Equal(uint64(0), works[id][1])
		} else if i < leaders { // 120 + 100 * 19
			require.Equal(uint64(100), works[id][0])
			require.Equal(uint64(100*(leaders-1)+100), works[id][1])
		} else if i < len(node.genesisNodes) { // 0 + 100 * 20
			require.Equal(uint64(0), works[id][0])
			require.Equal(uint64(100*leaders+100), works[id][1])
		} else { // 200 * 1.2 + 100 * 18
			require.Equal(uint64(200), works[id][0])
			require.Equal(uint64(100*(leaders-1)), works[node.IdForNetwork][1])
		}
	}
	offset, err = node.persistStore.ReadWorkOffset(node.IdForNetwork)
	require.Nil(err)
	require.Equal(uint64(1), offset)

	timestamp = uint64(clock.Now().Add(24 * time.Hour).UnixNano())
	snapshots = testBuildMintSnapshots(signers[1:], 2, timestamp)
	err = node.persistStore.WriteRoundWork(node.IdForNetwork, 2, snapshots[:10])
	require.Nil(err)
	for i := 1; i < leaders; i++ {
		err = node.persistStore.WriteRoundWork(signers[i], 2, snapshots[:10])
		require.Nil(err)
	}

	batch := (timestamp - node.Epoch) / (24 * uint64(time.Hour))
	for i, id := range signers {
		if i == leaders {
			break
		}
		err = node.persistStore.WriteRoundSpaceAndState(&common.RoundSpace{
			NodeId:   id,
			Batch:    batch,
			Round:    0,
			Duration: 0,
		})
		require.Nil(err)
	}

	accepted := make([]*CNode, len(signers))
	for i, id := range signers {
		accepted[i] = &CNode{IdForNetwork: id}
	}
	mints, err := node.distributeKernelMintByWorks(accepted, common.NewInteger(10000), timestamp)
	require.Nil(err)
	require.Len(mints, len(node.genesisNodes)+1)
	total := common.NewInteger(0)
	for i, m := range mints {
		if i == 0 { // 0
			require.Equal("52.72234781", m.Work.String())
		} else if i < leaders { // 1220 * 10
			require.Equal("369.22742985", m.Work.String())
		} else if i < len(node.genesisNodes) { // 1200 * 4
			require.Equal("366.41822348", m.Work.String())
		} else { // 1240
			require.Equal("369.83812689", m.Work.String())
		}
		total = total.Add(m.Work)
	}
	require.Equal(common.NewInteger(10000).Sub(total).String(), "0.00000016")
}

func testBuildMintSnapshots(signers []crypto.Hash, round, timestamp uint64) []*common.SnapshotWork {
	snapshots := make([]*common.SnapshotWork, 100)
	for i := range snapshots {
		hash := []byte(fmt.Sprintf("MW%d%d%d", round, timestamp, i))
		s := common.SnapshotWork{
			Timestamp: timestamp,
			Hash:      crypto.Blake3Hash(hash),
			Signers:   signers,
		}
		snapshots[i] = &s
	}
	return snapshots
}
