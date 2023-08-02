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
	"github.com/stretchr/testify/assert"
)

func TestPledgeAmount(t *testing.T) {
	assert := assert.New(t)

	for y, m := range map[int]string{
		0:  "10000",
		1:  "11000",
		2:  "11900",
		3:  "12710",
		4:  "13439",
		5:  "14095.1",
		6:  "14685.59",
		7:  "15217.031",
		8:  "15695.3279",
		9:  "16125.79511",
		10: "16513.215599",
	} {
		for b := 365 * y; b < 365*(y+1); b++ {
			since := time.Duration(b*24) * time.Hour
			assert.Equal(common.NewIntegerFromString(m), pledgeAmount(since))
		}
	}
}

func TestPoolSize(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(common.NewInteger(500000), poolSizeUniversal(0))
	assert.Equal(common.NewIntegerFromString("498630.13698640"), poolSizeUniversal(10))
	assert.Equal(common.NewInteger(500000), poolSizeUniversal(0))
	assert.Equal(common.NewIntegerFromString("450000"), poolSizeUniversal(365))
	assert.Equal(common.NewIntegerFromString("449876.71232877"), poolSizeUniversal(366))
}

func TestPoolSizeLegacy(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(common.NewInteger(500000), poolSizeLegacy(0))
	assert.Equal(common.NewIntegerFromString("498767.12328830"), poolSizeLegacy(10))
	assert.Equal(common.NewInteger(500000), poolSizeLegacy(0))
	assert.Equal(common.NewIntegerFromString("455000"), poolSizeLegacy(365))
	assert.Equal(common.NewIntegerFromString("454889.04109592"), poolSizeLegacy(366))
}

func TestUniversalMintTransaction(t *testing.T) {
	assert := assert.New(t)

	root, err := os.MkdirTemp("", "mixin-mint-test")
	assert.Nil(err)
	defer os.RemoveAll(root)
}

func TestMintWorks(t *testing.T) {
	assert := assert.New(t)

	root, err := os.MkdirTemp("", "mixin-mint-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	internal.ToggleMockRunAggregators(true)

	node := setupTestNode(assert, root)
	assert.NotNil(node)

	offset, err := node.persistStore.ReadWorkOffset(node.IdForNetwork)
	assert.Nil(err)
	assert.Equal(uint64(0), offset)

	signers := append(node.genesisNodes, node.IdForNetwork)
	timestamp := uint64(clock.Now().UnixNano())
	for i := 0; i < 2; i++ {
		snapshots := testBuildMintSnapshots(signers[1:], 0, timestamp)
		err = node.persistStore.WriteRoundWork(node.IdForNetwork, 0, snapshots)
		assert.Nil(err)
		for i := 1; i < 11; i++ {
			err = node.persistStore.WriteRoundWork(signers[i], 0, snapshots)
			assert.Nil(err)
		}

		works, err := node.persistStore.ListNodeWorks(signers, uint32(snapshots[0].Timestamp/uint64(time.Hour*24)))
		assert.Nil(err)
		assert.Len(works, 16)
		for i, id := range signers {
			if i == 0 {
				assert.Equal(uint64(0), works[id][0])
				assert.Equal(uint64(0), works[id][1])
			} else if i < 11 {
				assert.Equal(uint64(100), works[id][0])
				assert.Equal(uint64(1000), works[id][1])
			} else if i < 15 {
				assert.Equal(uint64(0), works[id][0])
				assert.Equal(uint64(1100), works[id][1])
			} else {
				assert.Equal(uint64(100), works[id][0])
				assert.Equal(uint64(1000), works[id][1])
			}
		}
		offset, err := node.persistStore.ReadWorkOffset(node.IdForNetwork)
		assert.Nil(err)
		assert.Equal(uint64(0), offset)
	}

	timestamp = uint64(clock.Now().UnixNano())
	snapshots := testBuildMintSnapshots(signers[1:], 1, timestamp)
	err = node.persistStore.WriteRoundWork(node.IdForNetwork, 1, snapshots[:98])
	assert.Nil(err)

	works, err := node.persistStore.ListNodeWorks(signers, uint32(snapshots[0].Timestamp/uint64(time.Hour*24)))
	assert.Nil(err)
	assert.Len(works, 16)
	assert.Equal(uint64(198), works[node.IdForNetwork][0])
	assert.Equal(uint64(1000), works[node.IdForNetwork][1])
	for i, id := range signers {
		if i == 0 {
			assert.Equal(uint64(0), works[id][0])
			assert.Equal(uint64(0), works[id][1])
		} else if i < 11 {
			assert.Equal(uint64(100), works[id][0])
			assert.Equal(uint64(1098), works[id][1])
		} else if i < 15 {
			assert.Equal(uint64(0), works[id][0])
			assert.Equal(uint64(1198), works[id][1])
		} else {
			assert.Equal(uint64(198), works[id][0])
			assert.Equal(uint64(1000), works[id][1])
		}
	}
	offset, err = node.persistStore.ReadWorkOffset(node.IdForNetwork)
	assert.Nil(err)
	assert.Equal(uint64(1), offset)

	err = node.persistStore.WriteRoundWork(node.IdForNetwork, 1, snapshots)
	assert.Nil(err)
	for i := 1; i < 11; i++ {
		err = node.persistStore.WriteRoundWork(signers[i], 1, nil)
		assert.Nil(err)
	}

	works, err = node.persistStore.ListNodeWorks(signers, uint32(snapshots[0].Timestamp/uint64(time.Hour*24)))
	assert.Nil(err)
	assert.Len(works, 16)
	assert.Equal(uint64(200), works[node.IdForNetwork][0])
	assert.Equal(uint64(1000), works[node.IdForNetwork][1])
	for i, id := range signers {
		if i == 0 {
			assert.Equal(uint64(0), works[id][0])
			assert.Equal(uint64(0), works[id][1])
		} else if i < 11 {
			assert.Equal(uint64(100), works[id][0])
			assert.Equal(uint64(1100), works[id][1])
		} else if i < 15 {
			assert.Equal(uint64(0), works[id][0])
			assert.Equal(uint64(1200), works[id][1])
		} else {
			assert.Equal(uint64(200), works[id][0])
			assert.Equal(uint64(1000), works[id][1])
		}
	}
	offset, err = node.persistStore.ReadWorkOffset(node.IdForNetwork)
	assert.Nil(err)
	assert.Equal(uint64(1), offset)

	timestamp = uint64(clock.Now().Add(24 * time.Hour).UnixNano())
	snapshots = testBuildMintSnapshots(signers[1:], 2, timestamp)
	err = node.persistStore.WriteRoundWork(node.IdForNetwork, 2, snapshots[:10])
	assert.Nil(err)
	for i := 1; i < 11; i++ {
		err = node.persistStore.WriteRoundWork(signers[i], 2, snapshots[:10])
		assert.Nil(err)
	}

	batch := (timestamp - node.Epoch) / (24 * uint64(time.Hour))
	for i, id := range signers {
		if i == 11 {
			break
		}
		err = node.persistStore.WriteRoundSpaceAndState(&common.RoundSpace{
			NodeId:   id,
			Batch:    batch,
			Round:    0,
			Duration: 0,
		})
		assert.Nil(err)
	}

	accepted := make([]*CNode, len(signers))
	for i, id := range signers {
		accepted[i] = &CNode{IdForNetwork: id}
	}
	mints, err := node.distributeKernelMintByWorks(accepted, common.NewInteger(10000), timestamp)
	assert.Nil(err)
	assert.Len(mints, 16)
	total := common.NewInteger(0)
	for i, m := range mints {
		if i == 0 { // 0
			assert.Equal("94.59529577", m.Work.String())
		} else if i < 11 { // 1220 * 10
			assert.Equal("662.58616354", m.Work.String())
		} else if i < 15 { // 1200 * 4
			assert.Equal("653.78520881", m.Work.String())
		} else { // 1240
			assert.Equal("664.40223356", m.Work.String())
		}
		total = total.Add(m.Work)
	}
	assert.True(common.NewInteger(10000).Sub(total).Cmp(common.NewIntegerFromString("0.0000001")) < 0)
}

func testBuildMintSnapshots(signers []crypto.Hash, round, timestamp uint64) []*common.SnapshotWork {
	snapshots := make([]*common.SnapshotWork, 100)
	for i := range snapshots {
		hash := []byte(fmt.Sprintf("MW%d%d%d", round, timestamp, i))
		s := common.SnapshotWork{
			Timestamp: timestamp,
			Hash:      crypto.NewHash(hash),
			Signers:   signers,
		}
		snapshots[i] = &s
	}
	return snapshots
}
