package kernel

import (
	"os"
	"testing"

	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/stretchr/testify/assert"
)

func TestDeterminBestRound(t *testing.T) {
	assert := assert.New(t)

	root, err := os.MkdirTemp("", "mixin-self-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	node := setupTestNode(assert, root)
	assert.NotNil(node)

	chain := node.GetOrCreateChain(node.IdForNetwork)
	best := chain.determinBestRound(uint64(clock.Now().UnixNano()))
	assert.Nil(best)

	chain = node.GetOrCreateChain(node.genesisNodes[0])
	best = chain.determinBestRound(uint64(clock.Now().UnixNano()))
	assert.NotNil(best)
}
