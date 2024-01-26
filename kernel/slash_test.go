package kernel

import (
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/stretchr/testify/require"
)

func TestNodeRemovalTime(t *testing.T) {
	require := require.New(t)

	root, err := os.MkdirTemp("", "mixin-mint-test")
	require.Nil(err)
	defer os.RemoveAll(root)

	node := setupTestNode(require, root)
	require.Equal(uint64(1551312000000000000), node.Epoch)
	now, ready := prepareNodeRemovalTime(node.Epoch, node.Epoch)
	require.False(ready)
	require.Equal(uint64(0), now)
	require.False(node.checkConsensusAcceptHour(now))

	now = node.Epoch + uint64(time.Second*3)
	now, ready = prepareNodeRemovalTime(now, node.Epoch)
	require.False(ready)
	require.Equal(uint64(0), now)
	require.False(node.checkConsensusAcceptHour(now))

	now, ready = prepareNodeRemovalTime(1706274368204746053, node.Epoch)
	require.True(ready)
	require.Equal(uint64(1706274060000000000), now)
	require.True(node.checkConsensusAcceptHour(now))

	for d := 0; d <= 3; d++ {
		for i := uint64(1); i <= config.KernelNodeAcceptTimeEnd; i++ {
			day := uint64(d * int(time.Hour) * 24)
			now := node.Epoch + uint64(time.Hour)*i + day
			now, ready = prepareNodeRemovalTime(now, node.Epoch)
			require.True(ready)
			hours := uint64(config.KernelNodeAcceptTimeBegin*time.Hour + time.Minute)
			require.Equal(node.Epoch+day+hours, now)
			require.True(node.checkConsensusAcceptHour(now))
		}
	}

	for d := 0; d <= 3; d++ {
		for i := uint64(config.KernelNodeAcceptTimeEnd + 1); i <= 23; i++ {
			day := uint64(d * int(time.Hour) * 24)
			now := node.Epoch + uint64(time.Hour)*i + day
			now, ready = prepareNodeRemovalTime(now, node.Epoch)
			require.False(ready)
			require.Equal(uint64(0), now)
			require.False(node.checkConsensusAcceptHour(now))
		}
	}
}
