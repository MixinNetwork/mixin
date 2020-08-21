package kernel

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestDeterminBestRound(t *testing.T) {
	assert := assert.New(t)

	root, err := ioutil.TempDir("", "mixin-self-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	node := setupTestNode(assert, root)
	assert.NotNil(node)

	chain := node.GetOrCreateChain(node.IdForNetwork)
	best, err := chain.determinBestRound(uint64(time.Now().UnixNano()), crypto.Hash{})
	assert.Nil(err)
	assert.Nil(best)

	chain = node.GetOrCreateChain(node.genesisNodes[0])
	best, err = chain.determinBestRound(uint64(time.Now().UnixNano()), crypto.Hash{})
	assert.NotNil(err)
	assert.Contains(err.Error(), "external hint not found in consensus")
	assert.Nil(best)

	chain = node.GetOrCreateChain(node.genesisNodes[0])
	best, err = chain.determinBestRound(uint64(time.Now().UnixNano()), node.IdForNetwork)
	assert.NotNil(err)
	assert.Contains(err.Error(), "external hint not found in consensus")
	assert.Nil(best)

	chain = node.GetOrCreateChain(node.genesisNodes[0])
	best, err = chain.determinBestRound(uint64(time.Now().UnixNano()), chain.ChainId)
	assert.Nil(err)
	assert.NotNil(best)
}
