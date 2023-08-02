package kernel

import (
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestRoundHash(t *testing.T) {
	require := require.New(t)

	nodeId := crypto.NewHash([]byte("node-test-id"))
	roundNumber := uint64(123)
	s1 := common.Snapshot{Version: common.SnapshotVersionCommonEncoding, Timestamp: 1663669260746463409}
	s2 := common.Snapshot{Version: common.SnapshotVersionCommonEncoding, Timestamp: 1663669260746463409 + uint64(2*time.Second)}
	snapshots := []*common.Snapshot{&s1, &s2}
	start, end, hash := ComputeRoundHash(nodeId, roundNumber, snapshots)
	require.Equal(uint64(1663669260746463409), start)
	require.Equal(uint64(1663669262746463409), end)
	require.Equal("c97ab71d9e3abf43214f5289049c94514fb41b5fcb9944dd6d0556717f1f7e81", hash.String())
}

func TestRoundHashLegacy(t *testing.T) {
	require := require.New(t)

	nodeId := crypto.NewHash([]byte("node-test-id"))
	roundNumber := uint64(123)
	s1 := common.Snapshot{Version: common.SnapshotVersionMsgpackEncoding, Timestamp: 1663669260746463409}
	s2 := common.Snapshot{Version: common.SnapshotVersionMsgpackEncoding, Timestamp: 1663669260746463409 + uint64(2*time.Second)}
	snapshots := []*common.Snapshot{&s1, &s2}
	start, end, hash := ComputeRoundHash(nodeId, roundNumber, snapshots)
	require.Equal(uint64(1663669260746463409), start)
	require.Equal(uint64(1663669262746463409), end)
	require.Equal("f0fecf0874977825e4d401d260674dd7661e8ac7167a6feaf4e31704c2582bd2", hash.String())
}
