package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	require := require.New(t)

	custom, err := Initialize("./config.example.toml")
	require.Nil(err)

	require.Equal("8bcfad3959892e8334fa287a3c9755fed017cd7a9e8c68d7540dc9e69fa4a00d", custom.Node.Signer.String())
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
