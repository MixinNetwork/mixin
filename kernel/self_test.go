package kernel

import (
	"os"
	"testing"

	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/stretchr/testify/require"
)

func TestDetermineBestRound(t *testing.T) {
	require := require.New(t)

	root, err := os.MkdirTemp("", "mixin-self-test")
	require.Nil(err)
	defer os.RemoveAll(root)

	node := setupTestNode(require, root)
	require.NotNil(node)

	chain := node.BootChain(node.IdForNetwork)
	best := chain.determineBestRound(clock.NowUnixNano())
	require.Nil(best)

	chain = node.BootChain(node.genesisNodes[0])
	best = chain.determineBestRound(clock.NowUnixNano())
	require.NotNil(best)
}
