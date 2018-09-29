package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/vmihailenco/msgpack"
	"gopkg.in/urfave/cli.v1"
)

func createAdressCmd(c *cli.Context) error {
	seed := make([]byte, 64)
	_, err := rand.Read(seed)
	if err != nil {
		return err
	}
	addr := common.NewAddressFromSeed(seed)
	fmt.Printf("address:\t%s\n", addr.String())
	fmt.Printf("view key:\t%s\n", addr.PrivateViewKey.String())
	fmt.Printf("spend key:\t%s\n", addr.PrivateSpendKey.String())
	return nil
}

func decodeTransactionCmd(c *cli.Context) error {
	raw, err := hex.DecodeString(c.String("raw"))
	if err != nil {
		return err
	}
	var tx common.SignedTransaction
	err = msgpack.Unmarshal(raw, &tx)
	if err != nil {
		return err
	}
	data, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func signTransactionCmd(c *cli.Context) error {
	return callRPC(c.String("node"), "signrawtransaction", []interface{}{
		c.String("raw"),
		c.String("key"),
	})
}

func sendTransactionCmd(c *cli.Context) error {
	return callRPC(c.String("node"), "sendrawtransaction", []interface{}{
		c.String("raw"),
	})
}

func listSnapshotsCmd(c *cli.Context) error {
	return callRPC(c.String("node"), "listsnapshots", []interface{}{
		c.Uint64("since"),
		c.Uint64("count"),
	})
}

func getSnapshotCmd(c *cli.Context) error {
	return callRPC(c.String("node"), "getsnapshot", []interface{}{
		c.String("hash"),
	})
}

func setupTestNetCmd(c *cli.Context) error {
	var accounts []common.Address

	for i := 0; i < 7; i++ {
		seed := make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			return err
		}
		accounts = append(accounts, common.NewAddressFromSeed(seed))
	}

	inputs := make([]map[string]string, 0)
	for _, a := range accounts {
		seed := make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			return err
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
		return err
	}
	fmt.Println(string(genesisData))

	nodes := make([]map[string]string, 0)
	for i, a := range accounts {
		nodes = append(nodes, map[string]string{
			"host":    fmt.Sprintf("127.0.0.1:700%d", i+1),
			"address": a.String(),
		})
	}
	nodesData, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(nodesData))

	for i, a := range accounts {
		dir := fmt.Sprintf("/tmp/mixin-700%d", i+1)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}

		store, err := storage.NewBadgerStore(dir)
		if err != nil {
			return err
		}
		err = store.StateSet("account", a)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(dir+"/genesis.json", genesisData, 0644)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(dir+"/nodes.json", nodesData, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

var httpClient *http.Client

func callRPC(node, method string, params []interface{}) error {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}

	body, err := json.Marshal(map[string]interface{}{
		"method": method,
		"params": params,
	})
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest("POST", "http://"+node, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Close = true
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
