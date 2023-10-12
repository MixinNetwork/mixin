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

	nodeId := crypto.Blake3Hash([]byte("node-test-id"))
	roundNumber := uint64(123)
	s1 := common.Snapshot{Version: common.SnapshotVersionCommonEncoding, Timestamp: 1663669260746463409}
	s2 := common.Snapshot{Version: common.SnapshotVersionCommonEncoding, Timestamp: 1663669260746463409 + uint64(2*time.Second)}
	snapshots := []*common.Snapshot{&s1, &s2}
	start, end, hash := ComputeRoundHash(nodeId, roundNumber, snapshots)
	require.Equal(uint64(1663669260746463409), start)
	require.Equal(uint64(1663669262746463409), end)
	require.Equal("b02daf53fbcbc2a3243b4b1e885cb9573531e491f4d92e16be08bb29f9a0a580", hash.String())
}
