package storage

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/MixinNetwork/mixin/config"
	"github.com/stretchr/testify/assert"
)

func TestBadger(t *testing.T) {
	assert := assert.New(t)
	err := config.Initialize("../config/config.example.toml")
	assert.Nil(err)

	root, err := ioutil.TempDir("", "mixin-badger-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	store, err := NewBadgerStore(root)
	assert.Nil(err)
	assert.NotNil(store)

	found, err := store.StateGet("state-key", nil)
	assert.Nil(err)
	assert.False(found)
	err = store.StateSet("state-key", 1)
	assert.Nil(err)
	var val int
	found, err = store.StateGet("state-key", &val)
	assert.Nil(err)
	assert.True(found)
	assert.Equal(1, val)

	seq := store.TopologySequence()
	assert.Equal(uint64(0), seq)

	err = store.Close()
	assert.Nil(err)
}
