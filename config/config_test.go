// +build ed25519 !custom_alg

package config

import (
	"testing"

	"github.com/MixinNetwork/mixin/crypto/ed25519"
	"github.com/stretchr/testify/assert"
)

func init() {
	ed25519.Load()
}

func TestConfig(t *testing.T) {
	assert := assert.New(t)

	custom, err := Initialize("./config.example.toml")
	assert.Nil(err)

	assert.Equal("56a7904a2dfd71c397bb48584033d8cb6ddcde9b46b7d91f07d2ede061723a0b", custom.Node.Signer.String())
	assert.Equal(false, custom.Node.ConsensusOnly)
	assert.Equal(700, custom.Node.KernelOprationPeriod)
	assert.Equal(16384, custom.Node.MemoryCacheSize)
	assert.Equal(7200, custom.Node.CacheTTL)
	assert.Equal(uint64(1048576), custom.Node.RingCacheSize)
	assert.Equal(uint64(16777216), custom.Node.RingFinalSize)
	assert.Equal("mixin-node.example.com:7239", custom.Network.Listener)
	assert.Equal(false, custom.RPC.Runtime)
}
