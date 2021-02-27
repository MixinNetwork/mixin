package kernel

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/stretchr/testify/assert"
)

func TestPledgeAmount(t *testing.T) {
	assert := assert.New(t)

	for b := 0; b < 365; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewInteger(10000), pledgeAmount(since))
	}
	for b := 365; b < 365*2; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewInteger(11000), pledgeAmount(since))
	}
	for b := 365 * 2; b < 365*3; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewInteger(11900), pledgeAmount(since))
	}
	for b := 365 * 3; b < 365*4; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewInteger(12710), pledgeAmount(since))
	}
	for b := 365 * 5; b < 365*6; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewIntegerFromString("14095.1"), pledgeAmount(since))
	}
	for b := 365 * 7; b < 365*8; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewIntegerFromString("15217.031"), pledgeAmount(since))
	}
	for b := 365 * 10; b < 365*11; b++ {
		since := time.Duration(b*24) * time.Hour
		assert.Equal(common.NewIntegerFromString("16513.215599"), pledgeAmount(since))
	}
}

func TestPoolSize(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(common.NewInteger(500000), poolSize(0))
	assert.Equal(common.NewIntegerFromString("498767.12328830"), poolSize(10))
	assert.Equal(common.NewInteger(500000), poolSize(0))
	assert.Equal(common.NewIntegerFromString("455000"), poolSize(365))
	assert.Equal(common.NewIntegerFromString("454889.04109592"), poolSize(366))
}

func TestMintWorks(t *testing.T) {
	assert := assert.New(t)

	root, err := ioutil.TempDir("", "mixin-mint-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	node := setupTestNode(assert, root)
	assert.NotNil(node)

	offset, err := node.persistStore.ReadWorkOffset(node.IdForNetwork)
	assert.Nil(err)
	assert.Equal(uint64(0), offset)

	signers := append(node.genesisNodes, node.IdForNetwork)
	timestamp := uint64(clock.Now().UnixNano())
	for i := 0; i < 2; i++ {
		snapshots := testBuildMintSnapshots(node.IdForNetwork, signers[1:], 0, timestamp)
		err = node.persistStore.WriteRoundWork(node.IdForNetwork, 0, snapshots)
		assert.Nil(err)

		works, err := node.persistStore.ListNodeWorks(signers, uint32(snapshots[0].Timestamp/uint64(time.Hour*24)))
		assert.Nil(err)
		assert.Len(works, 16)
		assert.Equal(uint64(100), works[node.IdForNetwork][0])
		assert.Equal(uint64(0), works[node.IdForNetwork][1])
		for i, id := range node.genesisNodes {
			assert.Equal(uint64(0), works[id][0])
			if i == 0 {
				assert.Equal(uint64(0), works[id][1])
			} else {
				assert.Equal(uint64(100), works[id][1])
			}
		}
		offset, err := node.persistStore.ReadWorkOffset(node.IdForNetwork)
		assert.Nil(err)
		assert.Equal(uint64(0), offset)
	}

	timestamp = uint64(clock.Now().UnixNano())
	snapshots := testBuildMintSnapshots(node.IdForNetwork, signers[1:], 1, timestamp)
	err = node.persistStore.WriteRoundWork(node.IdForNetwork, 1, snapshots[:98])
	assert.Nil(err)

	works, err := node.persistStore.ListNodeWorks(signers, uint32(snapshots[0].Timestamp/uint64(time.Hour*24)))
	assert.Nil(err)
	assert.Len(works, 16)
	assert.Equal(uint64(198), works[node.IdForNetwork][0])
	assert.Equal(uint64(0), works[node.IdForNetwork][1])
	for i, id := range node.genesisNodes {
		assert.Equal(uint64(0), works[id][0])
		if i == 0 {
			assert.Equal(uint64(0), works[id][1])
		} else {
			assert.Equal(uint64(198), works[id][1])
		}
	}
	offset, err = node.persistStore.ReadWorkOffset(node.IdForNetwork)
	assert.Nil(err)
	assert.Equal(uint64(1), offset)

	err = node.persistStore.WriteRoundWork(node.IdForNetwork, 1, snapshots)
	assert.Nil(err)

	works, err = node.persistStore.ListNodeWorks(signers, uint32(snapshots[0].Timestamp/uint64(time.Hour*24)))
	assert.Nil(err)
	assert.Len(works, 16)
	assert.Equal(uint64(200), works[node.IdForNetwork][0])
	assert.Equal(uint64(0), works[node.IdForNetwork][1])
	for i, id := range node.genesisNodes {
		assert.Equal(uint64(0), works[id][0])
		if i == 0 {
			assert.Equal(uint64(0), works[id][1])
		} else {
			assert.Equal(uint64(200), works[id][1])
		}
	}
	offset, err = node.persistStore.ReadWorkOffset(node.IdForNetwork)
	assert.Nil(err)
	assert.Equal(uint64(1), offset)

	timestamp = uint64(clock.Now().Add(24 * time.Hour).UnixNano())
	snapshots = testBuildMintSnapshots(node.IdForNetwork, signers[1:], 2, timestamp)
	err = node.persistStore.WriteRoundWork(node.IdForNetwork, 2, snapshots[:10])
	assert.Nil(err)

	accepted := make([]*CNode, len(signers))
	for i, id := range signers {
		accepted[i] = &CNode{IdForNetwork: id}
	}
	mints, err := node.distributeMintByWorks(accepted, common.NewInteger(10000), timestamp)
	assert.Nil(err)
	assert.Len(mints, 16)
	total := common.NewInteger(0)
	for i, m := range mints {
		if i == 0 {
			assert.Equal("94.13241292", m.Work.String())
		} else if m.IdForNetwork == node.IdForNetwork {
			assert.Equal("680.89112018", m.Work.String())
		} else {
			assert.Equal("658.92689049", m.Work.String())
		}
		total = total.Add(m.Work)
	}
	assert.True(common.NewInteger(10000).Sub(total).Cmp(common.NewIntegerFromString("0.0000001")) < 0)
}

func testBuildMintSnapshots(nodeId crypto.Hash, signers []crypto.Hash, round, timestamp uint64) []*common.SnapshotWithTopologicalOrder {
	snapshots := make([]*common.SnapshotWithTopologicalOrder, 100)
	for i := range snapshots {
		hash := []byte(fmt.Sprintf("MW%d%d%d", round, timestamp, i))
		s := common.Snapshot{
			NodeId:      nodeId,
			RoundNumber: round,
			Timestamp:   timestamp,
			Hash:        crypto.NewHash(hash),
		}
		snapshots[i] = &common.SnapshotWithTopologicalOrder{
			Snapshot: s,
			Signers:  signers,
		}
	}
	return snapshots
}
