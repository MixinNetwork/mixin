package kernel

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/VictoriaMetrics/fastcache"
	"github.com/stretchr/testify/assert"
)

func TestNodeRemovePossibility(t *testing.T) {
	assert := assert.New(t)

	root, err := ioutil.TempDir("", "mixin-election-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	node := setupTestNode(assert, root)
	assert.NotNil(node)

	now, err := time.Parse(time.RFC3339, "2020-02-09T15:35:13Z")
	assert.Nil(err)
	candi, err := node.checkRemovePossibility(uint64(now.UnixNano()))
	assert.Nil(candi)
	assert.NotNil(err)
	assert.Contains(err.Error(), "all old nodes removed")

	now, err = time.Parse(time.RFC3339, "2020-02-27T17:00:00Z")
	assert.Nil(err)
	candi, err = node.checkRemovePossibility(uint64(now.UnixNano()))
	assert.Nil(candi)
	assert.NotNil(err)
	assert.Contains(err.Error(), "all old nodes removed")

	now, err = time.Parse(time.RFC3339, "2020-02-28T00:00:00Z")
	assert.Nil(err)
	candi, err = node.checkRemovePossibility(uint64(now.UnixNano()))
	assert.Nil(candi)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid node remove hour")

	now, err = time.Parse(time.RFC3339, "2020-02-28T17:00:00Z")
	assert.Nil(err)
	candi, err = node.checkRemovePossibility(uint64(now.UnixNano()))
	assert.Nil(err)
	assert.NotNil(candi)
}

func setupTestNode(assert *assert.Assertions, dir string) *Node {
	data, err := ioutil.ReadFile("../config/config.example.json")
	assert.Nil(err)
	err = ioutil.WriteFile(dir+"/config.json", data, 0644)
	assert.Nil(err)

	data, err = ioutil.ReadFile("../config/genesis.json")
	assert.Nil(err)
	err = ioutil.WriteFile(dir+"/genesis.json", data, 0644)
	assert.Nil(err)

	data, err = ioutil.ReadFile("../config/nodes.json")
	assert.Nil(err)
	err = ioutil.WriteFile(dir+"/nodes.json", data, 0644)
	assert.Nil(err)

	config.Initialize(dir + "/config.json")
	cache := fastcache.New(16 * 1024 * 1024)
	store, err := storage.NewBadgerStore(dir)
	assert.Nil(err)
	assert.NotNil(store)
	node, err := SetupNode(store, cache, ":7239", dir)
	assert.Nil(err)
	return node
}
