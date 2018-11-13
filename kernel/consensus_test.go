package kernel

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/stretchr/testify/assert"
)

func TestConsensus(t *testing.T) {
	assert := assert.New(t)

	root, err := ioutil.TempDir("", "mixin-consensus-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	accounts, err := setupTestNet(root)
	assert.Nil(err)
	assert.Len(accounts, 7)

	stores := make([]storage.Store, 0)
	for i, _ := range accounts {
		dir := fmt.Sprintf(root+"/700%d", i+1)
		store, err := storage.NewBadgerStore(dir)
		assert.Nil(err)
		stores = append(stores, store)
		go Loop(store, fmt.Sprintf("127.0.0.1:700%d", i+1), dir)
	}

	time.Sleep(3000 * time.Second)
}

func setupTestNet(root string) ([]common.Address, error) {
	var accounts []common.Address

	for i := 0; i < 7; i++ {
		seed := make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			return accounts, err
		}
		accounts = append(accounts, common.NewAddressFromSeed(seed))
	}

	inputs := make([]map[string]string, 0)
	for _, a := range accounts {
		seed := make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			return accounts, err
		}
		mask := crypto.NewKeyFromSeed(seed)
		inputs = append(inputs, map[string]string{
			"address": a.String(),
			"balance": "20000",
			"mask":    mask.String(),
		})
	}
	genesisData, err := json.MarshalIndent(inputs, "", "  ")
	if err != nil {
		return accounts, err
	}

	nodes := make([]map[string]string, 0)
	for i, a := range accounts {
		nodes = append(nodes, map[string]string{
			"host":    fmt.Sprintf("127.0.0.1:700%d", i+1),
			"address": a.String(),
		})
	}
	nodesData, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return accounts, err
	}

	for i, a := range accounts {
		dir := fmt.Sprintf(root+"/700%d", i+1)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return accounts, err
		}

		store, err := storage.NewBadgerStore(dir)
		if err != nil {
			return accounts, err
		}
		err = store.StateSet("account", a)
		if err != nil {
			return accounts, err
		}

		err = ioutil.WriteFile(dir+"/genesis.json", genesisData, 0644)
		if err != nil {
			return accounts, err
		}
		err = ioutil.WriteFile(dir+"/nodes.json", nodesData, 0644)
		if err != nil {
			return accounts, err
		}

		err = store.Close()
		if err != nil {
			return accounts, err
		}
	}
	return accounts, nil
}
