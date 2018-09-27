package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

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
	store, err := storage.NewBadgerStore(c.String("dir"))
	if err != nil {
		return err
	}

	var raw struct {
		Inputs []struct {
			Hash  crypto.Hash `json:"hash"`
			Index int         `json:"index"`
		} `json:"inputs"`
		Outputs []struct {
			Type     uint8            `json:"type"`
			Script   common.Script    `json:"script"`
			Accounts []common.Address `json:"accounts"`
			Amount   common.Integer   `json:"amount"`
		}
		Asset crypto.Hash `json:"asset"`
		Extra string      `json:"extra"`
	}
	err = json.Unmarshal([]byte(c.String("raw")), &raw)
	if err != nil {
		return err
	}

	tx := common.NewTransaction(raw.Asset)
	for _, in := range raw.Inputs {
		tx.AddInput(in.Hash, in.Index)
	}

	for _, out := range raw.Outputs {
		if out.Type != common.OutputTypeScript {
			return fmt.Errorf("invalid output type %d", out.Type)
		}
		tx.AddScriptOutput(out.Accounts, out.Script, out.Amount)
	}

	extra, err := hex.DecodeString(raw.Extra)
	if err != nil {
		return err
	}
	tx.Extra = extra

	key, err := hex.DecodeString(c.String("key"))
	if err != nil {
		return err
	}
	if len(key) != 64 {
		return fmt.Errorf("invalid key length %d", len(key))
	}
	var account common.Address
	copy(account.PrivateViewKey[:], key[:32])
	copy(account.PrivateSpendKey[:], key[32:])

	signed := &common.SignedTransaction{Transaction: *tx}
	for i, _ := range signed.Inputs {
		err := signed.SignInput(store.SnapshotsGetUTXO, i, []common.Address{account})
		if err != nil {
			return err
		}
	}
	fmt.Println(hex.EncodeToString(signed.Marshal()))
	return signed.Validate(store.SnapshotsGetUTXO, store.SnapshotsCheckGhost)
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

func spendGenesisTestAmount(c *cli.Context) error {
	return nil
}
