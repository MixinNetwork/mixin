package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	assert := assert.New(t)

	err := Initialize("./config.example.toml")
	assert.Nil(err)

	assert.Equal("56a7904a2dfd71c397bb48584033d8cb6ddcde9b46b7d91f07d2ede061723a0b", Custom.Node.Signer.String())
	assert.Equal(true, Custom.Node.ConsensusOnly)
	assert.Equal(700, Custom.Node.KernelOprationPeriod)
	assert.Equal(16384, Custom.Node.MemoryCacheSize)
	assert.Equal(7200, Custom.Node.CacheTTL)
	assert.Equal(uint64(1048576), Custom.Node.RingCacheSize)
	assert.Equal(uint64(16777216), Custom.Node.RingFinalSize)
	assert.Equal("mixin-node.example.com:7239", Custom.Network.Listener)
	assert.Equal(false, Custom.RPC.Runtime)
}
