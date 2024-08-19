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
	require.Equal(1024, custom.Node.MemoryCacheSize)
	require.Equal(3600, custom.Node.CacheTTL)

	require.Equal(true, custom.Storage.ValueLogGC)
	require.Equal(7, custom.Storage.MaxCompactionLevels)

	require.Equal(false, custom.P2P.Relayer)
	require.Len(custom.P2P.Seeds, 4)
	require.Equal("06ff8589d5d8b40dd90a8120fa65b273d136ba4896e46ad20d76e53a9b73fd9f@seed.mixin.dev:5850", custom.P2P.Seeds[0])
	require.Equal(false, custom.RPC.Runtime)
}
