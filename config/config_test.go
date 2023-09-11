package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	require := require.New(t)

	custom, err := Initialize("./config.example.toml")
	require.Nil(err)

	require.Equal("56a7904a2dfd71c397bb48584033d8cb6ddcde9b46b7d91f07d2ede061723a0b", custom.Node.Signer.String())
	require.Equal(false, custom.Node.ConsensusOnly)
	require.Equal(700, custom.Node.KernelOprationPeriod)
	require.Equal(4096, custom.Node.MemoryCacheSize)
	require.Equal(7200, custom.Node.CacheTTL)

	require.Equal(true, custom.Storage.ValueLogGC)
	require.Equal(7, custom.Storage.MaxCompactionLevels)

	require.Equal("mixin-node.example.com:7239", custom.Network.Listener)
	require.Len(custom.Network.Peers, 27)
	require.Equal("lehigh-2.hotot.org:7239", custom.Network.Peers[26])
	require.Equal(false, custom.RPC.Runtime)
}
