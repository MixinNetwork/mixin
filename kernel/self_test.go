package kernel

import (
	"testing"

	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/stretchr/testify/require"
)

func TestDetermineBestRound(t *testing.T) {
	require := require.New(t)

	root := t.TempDir()

	node := setupTestNode(require, root)
	require.NotNil(node)

	chain := node.BootChain(node.IdForNetwork)
	best := chain.determineBestRound(clock.NowUnixNano())
	require.Nil(best)

	chain = node.BootChain(node.genesisNodes[0])
	best = chain.determineBestRound(clock.NowUnixNano())
	require.NotNil(best)
}
