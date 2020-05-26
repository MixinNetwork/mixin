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
	custom, err := config.Initialize(configFilePath)
	assert.Nil(err)

	root, err := ioutil.TempDir("", "mixin-badger-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	store, err := NewBadgerStore(custom, root)
	assert.Nil(err)
	assert.NotNil(store)

	seq := store.TopologySequence()
	assert.Equal(uint64(0), seq)

	err = store.Close()
	assert.Nil(err)
}
