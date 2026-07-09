package kernel

import (
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestResolveLegacyWantTxs(t *testing.T) {
	require := require.New(t)

	snap := crypto.Blake3Hash([]byte("snapshot"))
	tx := crypto.Blake3Hash([]byte("transaction"))
	explicit := crypto.Blake3Hash([]byte("explicit"))
	node := &Node{
		chain: &Chain{
			CosiAggregators: map[crypto.Hash]*CosiAggregator{
				snap: {Snapshot: &common.Snapshot{Transactions: []crypto.Hash{tx}}},
			},
		},
	}

	require.Nil(node.resolveLegacyWantTxs(snap, nil))
	require.Equal([]crypto.Hash{explicit}, node.resolveLegacyWantTxs(snap, []crypto.Hash{explicit}))
	require.Equal([]crypto.Hash{tx}, node.resolveLegacyWantTxs(snap, []crypto.Hash{}))
}
