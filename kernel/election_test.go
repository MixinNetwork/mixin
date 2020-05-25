package kernel

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
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
	candi, err := node.checkRemovePossibility(node.IdForNetwork, uint64(now.UnixNano()))
	assert.Nil(candi)
	assert.NotNil(err)
	assert.Contains(err.Error(), "all old nodes removed")

	now, err = time.Parse(time.RFC3339, "2020-02-27T17:00:00Z")
	assert.Nil(err)
	candi, err = node.checkRemovePossibility(node.IdForNetwork, uint64(now.UnixNano()))
	assert.Nil(candi)
	assert.NotNil(err)
	assert.Contains(err.Error(), "all old nodes removed")

	now, err = time.Parse(time.RFC3339, "2020-02-28T00:00:00Z")
	assert.Nil(err)
	candi, err = node.checkRemovePossibility(node.IdForNetwork, uint64(now.UnixNano()))
	assert.Nil(candi)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid node remove hour")

	now, err = time.Parse(time.RFC3339, "2020-02-28T17:00:00Z")
	assert.Nil(err)
	candi, err = node.checkRemovePossibility(node.IdForNetwork, uint64(now.UnixNano()))
	assert.Nil(err)
	assert.NotNil(candi)
	assert.Equal("028d97996a0b78f48e43f90e82137dbca60199519453a8fbf6e04b1e4d11efc9", candi.IdForNetwork(node.networkId).String())
	assert.Nil(node.ConsensusRemovedRecently(uint64(now.UnixNano())))

	tx, err := node.buildRemoveTransaction(candi)
	assert.Nil(err)
	assert.NotNil(tx)
	assert.Equal("d5af53561d99eb52af2b98b57d3fb0cc8ae4c6449ec6c89d8427201051a947a2", tx.PayloadHash().String())
	assert.Equal(common.XINAssetId, tx.Asset)
	assert.Equal(pledgeAmount(0), tx.Outputs[0].Amount)
	assert.Equal("fffe01", tx.Outputs[0].Script.String())
	assert.Equal(uint8(common.OutputTypeNodeRemove), tx.Outputs[0].Type)
	assert.Equal(uint8(common.TransactionTypeNodeRemove), tx.TransactionType())
	assert.Len(tx.Outputs[0].Keys, 1)

	err = tx.SignInput(node.persistStore, 0, []common.Address{node.Signer})
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid key for the input")
	err = tx.Validate(node.persistStore)
	assert.Nil(err)

	payee, err := common.NewAddressFromString("XINYDpVHXHxkFRPbP9LZak5p7FZs3mWTeKvrAzo4g9uziTW99t7LrU7me66Xhm6oXGTbYczQLvznk3hxgNSfNBaZveAmEeRM")
	assert.Nil(err)
	mask, err := tx.Outputs[0].Mask.AsPublicKey()
	assert.Nil(err)
	ghost, err := tx.Outputs[0].Keys[0].AsPublicKey()
	assert.Nil(err)
	view := payee.PublicSpendKey.DeterministicHashDerive()
	assert.Equal(payee.PublicSpendKey.String(), crypto.ViewGhostOutputKey(mask, ghost, view, 0).String())
}

var configData = []byte(`[node]
signer-key = "56a7904a2dfd71c397bb48584033d8cb6ddcde9b46b7d91f07d2ede061723a0b"
consensus-only = true
memory-cache-size = 16
cache-ttl = 7200
ring-cache-size = 4096
ring-final-size = 16384
[network]
listener = "mixin-node.example.com:7239"`)

func setupTestNode(assert *assert.Assertions, dir string) *Node {
	err := ioutil.WriteFile(dir+"/config.toml", configData, 0644)
	assert.Nil(err)

	data, err := ioutil.ReadFile("../config/genesis.json")
	assert.Nil(err)
	err = ioutil.WriteFile(dir+"/genesis.json", data, 0644)
	assert.Nil(err)

	data, err = ioutil.ReadFile("../config/nodes.json")
	assert.Nil(err)
	err = ioutil.WriteFile(dir+"/nodes.json", data, 0644)
	assert.Nil(err)

	custom, err := config.Initialize(dir + "/config.toml")
	assert.Nil(err)
	cache := fastcache.New(16 * 1024 * 1024)
	store, err := storage.NewBadgerStore(custom, dir)
	assert.Nil(err)
	assert.NotNil(store)
	node, err := SetupNode(custom, store, cache, ":7239", dir)
	assert.Nil(err)
	return node
}
