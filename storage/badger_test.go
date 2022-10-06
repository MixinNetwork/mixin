package storage

import (
	"os"
	"testing"

	"github.com/MixinNetwork/mixin/config"
	"github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/assert"
)

func TestBadger(t *testing.T) {
	assert := assert.New(t)
	custom, err := config.Initialize("../config/config.example.toml")
	assert.Nil(err)

	root, err := os.MkdirTemp("", "mixin-badger-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	store, err := NewBadgerStore(custom, root)
	assert.Nil(err)
	assert.NotNil(store)

	seq := store.TopologySequence()
	assert.Equal(uint64(0), seq)

	err = store.snapshotsDB.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte("key-not-found"))
	})
	assert.Nil(err)

	err = store.Close()
	assert.Nil(err)
}
