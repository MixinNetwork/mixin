package kernel

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

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
	best := chain.determinBestRound(uint64(time.Now().UnixNano()))
	assert.Nil(best)

	chain = node.GetOrCreateChain(node.genesisNodes[0])
	best = chain.determinBestRound(uint64(time.Now().UnixNano()))
	assert.NotNil(best)
}
