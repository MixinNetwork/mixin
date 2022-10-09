package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	assert := assert.New(t)

	custom, err := Initialize("./config.example.toml")
	assert.Nil(err)

	assert.Equal("56a7904a2dfd71c397bb48584033d8cb6ddcde9b46b7d91f07d2ede061723a0b", custom.Node.Signer.String())
	assert.Equal(false, custom.Node.ConsensusOnly)
	assert.Equal(700, custom.Node.KernelOprationPeriod)
	assert.Equal(4096, custom.Node.MemoryCacheSize)
	assert.Equal(7200, custom.Node.CacheTTL)

	assert.Equal("mixin-node.example.com:7239", custom.Network.Listener)
	assert.Len(custom.Network.Peers, 26)
	assert.Equal("mixin-node-04.b.watch:7239", custom.Network.Peers[23])
	assert.Equal(false, custom.RPC.Runtime)
}
