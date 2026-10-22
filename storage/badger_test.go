package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/util"
	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/require"
)

func TestBadger(t *testing.T) {
	require := require.New(t)
	custom, err := config.Initialize("../config/config.example.toml")
	require.Nil(err)

	root := t.TempDir()

	store, err := NewBadgerStore(custom, root)
	require.Nil(err)
	require.NotNil(store)
	defer util.CloseOrPanic(store)

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte("key-not-found"))
	})
	require.Nil(err)

	err = store.Close()
	require.Nil(err)
}

func TestNewBadgerStoreReleasesSnapshotsOnCacheOpenFailure(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache")
	require.NoError(t, os.WriteFile(cachePath, nil, 0o600))
	store, err := NewBadgerStore(nil, dir)
	require.Error(t, err)
	require.Nil(t, store)

	require.NoError(t, os.Remove(cachePath))
	store, err = NewBadgerStore(nil, dir)
	require.NoError(t, err)
	require.NoError(t, store.Close())
}
