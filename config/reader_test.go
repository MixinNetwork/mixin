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
	require.Equal(700, custom.Node.KernelOprationPeriod)
	require.Equal(4096, custom.Node.MemoryCacheSize)
	require.Equal(7200, custom.Node.CacheTTL)

	require.Equal(true, custom.Storage.ValueLogGC)
	require.Equal(7, custom.Storage.MaxCompactionLevels)

	require.Equal(false, custom.Network.Relayer)
	require.Len(custom.Network.Peers, 3)
	require.Equal("5e7ca75239ff68231bd0bcebc8be5b4725e8784b4df8788306c9baa291ec8595@mixin-node1.b1.run:7239", custom.Network.Peers[2])
	require.Equal(false, custom.RPC.Runtime)
}
