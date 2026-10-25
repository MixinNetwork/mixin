package storage

import (
	"bytes"
	"testing"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestReadUTXOKeysMissingInput(t *testing.T) {
	store := newTestBadgerStore(t)
	hash := crypto.Blake3Hash([]byte("missing input"))
	keys, err := store.ReadUTXOKeys(hash, 0)
	require.NoError(t, err)
	require.Nil(t, keys)

	account := common.NewAddressFromSeed(bytes.Repeat([]byte{23}, 64))
	tx := common.NewTransactionV5(common.XINAssetId)
	tx.AddInput(hash, 0)
	require.ErrorContains(t, tx.AsVersioned().SignInput(store, 0, []*common.Address{&account}), "input not found")
}
