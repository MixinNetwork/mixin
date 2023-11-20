package storage

import (
	"os"
	"testing"

	"github.com/MixinNetwork/mixin/config"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestBadger(t *testing.T) {
	require := require.New(t)
	custom, err := config.Initialize("../config/config.example.toml")
	require.Nil(err)

	root, err := os.MkdirTemp("", "mixin-badger-test")
	require.Nil(err)
	defer os.RemoveAll(root)

	store, err := NewBadgerStore(custom, root)
	require.Nil(err)
	require.NotNil(store)
	defer store.Close()

	seq := store.TopologySequence()
	require.Equal(uint64(0), seq)

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte("key-not-found"))
	})
	require.Nil(err)

	err = store.Close()
	require.Nil(err)
}
