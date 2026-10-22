package config

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigParsing(t *testing.T) {
	require := require.New(t)

	custom, err := Initialize("./config.example.toml")
	require.Nil(err)

	require.Equal("8bcfad3959892e8334fa287a3c9755fed017cd7a9e8c68d7540dc9e69fa4a00d", custom.Node.Signer.String())
	require.Equal(700, custom.Node.KernelOperationPeriod)
	require.Equal(512, custom.Node.MemoryCacheSize)
	require.Equal(3600, custom.Node.CacheTTL)

	require.Equal(true, custom.Storage.ValueLogGC)
	require.Equal(7, custom.Storage.MaxCompactionLevels)

	require.Equal(false, custom.P2P.Relayer)
	require.Len(custom.P2P.Seeds, 4)
	require.Equal("06ff8589d5d8b40dd90a8120fa65b273d136ba4896e46ad20d76e53a9b73fd9f@seed.mixin.dev:5850", custom.P2P.Seeds[0])
	require.Equal(false, custom.RPC.Runtime)
}

func TestConfigDefaultsAndNumericBounds(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.toml")
	const node = "[node]\nsigner-key = \"8bcfad3959892e8334fa287a3c9755fed017cd7a9e8c68d7540dc9e69fa4a00d\"\n"
	for _, fields := range []string{"", "kernel-operation-period = 0\nmemory-cache-size = 0\ncache-ttl = 0\n"} {
		require.NoError(t, os.WriteFile(file, []byte(node+fields), 0o600))
		custom, err := Initialize(file)
		require.NoError(t, err)
		require.Equal(t, 700, custom.Node.KernelOperationPeriod)
		require.Equal(t, 4096, custom.Node.MemoryCacheSize)
		require.Equal(t, 7200, custom.Node.CacheTTL)
	}

	maxSeconds := int64(math.MaxInt64) / int64(time.Second)
	for _, tc := range []struct {
		field string
		value int64
	}{
		{"kernel-operation-period", -1},
		{"kernel-operation-period", maxSeconds + 1},
		{"memory-cache-size", -1},
		{"memory-cache-size", int64(math.MaxInt/(1024*1024)) + 1},
		{"cache-ttl", -1},
		{"cache-ttl", maxSeconds - 59},
		{"cache-ttl", int64(math.MaxInt)},
	} {
		t.Run(fmt.Sprintf("%s=%d", tc.field, tc.value), func(t *testing.T) {
			data := node + fmt.Sprintf("%s = %d\n", tc.field, tc.value)
			require.NoError(t, os.WriteFile(file, []byte(data), 0o600))
			_, err := Initialize(file)
			require.ErrorContains(t, err, tc.field)
		})
	}

	data := node + fmt.Sprintf("kernel-operation-period = %d\ncache-ttl = %d\nmemory-cache-size = %d\n",
		min(maxSeconds, int64(math.MaxInt)), min(maxSeconds-60, int64(math.MaxInt)-60), math.MaxInt/(1024*1024))
	require.NoError(t, os.WriteFile(file, []byte(data), 0o600))
	_, err := Initialize(file)
	require.NoError(t, err)
}

func TestConfigRejectsNonCanonicalSigner(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.toml")
	data := "[node]\nsigner-key = \"" + strings.Repeat("ff", 32) + "\"\n"
	require.NoError(t, os.WriteFile(file, []byte(data), 0o600))
	_, err := Initialize(file)
	require.ErrorContains(t, err, "invalid node signer-key")
}
