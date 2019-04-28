package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
	"gopkg.in/urfave/cli.v1"
)

func createAdressCmd(c *cli.Context) error {
	seed := make([]byte, 64)
	_, err := rand.Read(seed)
	if err != nil {
		return err
	}
	addr := common.NewAddressFromSeed(seed)
	if c.Bool("public") {
		addr.PrivateViewKey = addr.PublicSpendKey.DeterministicHashDerive()
		addr.PublicViewKey = addr.PrivateViewKey.Public()
	}
	if view := c.String("view"); len(view) > 0 {
		key, err := hex.DecodeString(view)
		if err != nil {
			return err
		}
		copy(addr.PrivateViewKey[:], key)
		addr.PublicViewKey = addr.PrivateViewKey.Public()
	}
	if spend := c.String("spend"); len(spend) > 0 {
		key, err := hex.DecodeString(spend)
		if err != nil {
			return err
		}
		copy(addr.PrivateSpendKey[:], key)
		addr.PublicSpendKey = addr.PrivateSpendKey.Public()
	}
	fmt.Printf("address:\t%s\n", addr.String())
	fmt.Printf("view key:\t%s\n", addr.PrivateViewKey.String())
	fmt.Printf("spend key:\t%s\n", addr.PrivateSpendKey.String())
	return nil
}

func updateHeadReference(c *cli.Context) error {
	store, err := storage.NewBadgerStore(c.String("dir"))
	if err != nil {
		return err
	}
	defer store.Close()
	node, err := crypto.HashFromString(c.String("node"))
	if err != nil {
		return err
	}
	round, err := store.ReadRound(node)
	if err != nil {
		return err
	}
	if round == nil {
		return errors.New("node not found")
	}
	fmt.Printf("node: %s round: %d self: %s external: %s\n", round.NodeId.String(), round.Number, round.References.Self.String(), round.References.External.String())
	if round.Number != c.Uint64("round") {
		return fmt.Errorf("round number not match %d", round.Number)
	}
	external, err := hex.DecodeString(c.String("external"))
	if err != nil {
		return err
	}
	copy(round.References.External[:], external)
	return store.UpdateEmptyHeadRound(round.NodeId, round.Number, round.References)
}

func removeGraphEntries(c *cli.Context) error {
	store, err := storage.NewBadgerStore(c.String("dir"))
	if err != nil {
		return err
	}
	defer store.Close()
	return store.RemoveGraphEntries(c.String("prefix"))
}

func validateGraphEntries(c *cli.Context) error {
	store, err := storage.NewBadgerStore(c.String("dir"))
	if err != nil {
		return err
	}
	defer store.Close()
	invalid, err := store.ValidateGraphEntries()
	if err != nil {
		return err
	}
	fmt.Printf("invalid entries: %d\n", invalid)
	return nil
}

func decodeTransactionCmd(c *cli.Context) error {
	raw, err := hex.DecodeString(c.String("raw"))
	if err != nil {
		return err
	}
	var tx common.SignedTransaction
	err = common.MsgpackUnmarshal(raw, &tx)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func signTransactionCmd(c *cli.Context) error {
	var raw signerInput
	err := json.Unmarshal([]byte(c.String("raw")), &raw)
	if err != nil {
		return err
	}
	raw.Node = c.String("node")

	seed, err := hex.DecodeString(c.String("seed"))
	if err != nil {
		return err
	}
	if len(seed) != 64 {
		seed = make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			return err
		}
	}

	tx := common.NewTransaction(raw.Asset)
	for _, in := range raw.Inputs {
		if in.Deposit != nil {
			tx.AddDepositInput(in.Deposit)
		} else {
			tx.AddInput(in.Hash, in.Index)
		}
	}

	for _, out := range raw.Outputs {
		if out.Type != common.OutputTypeScript {
			return fmt.Errorf("invalid output type %d", out.Type)
		}
		tx.AddScriptOutput(out.Accounts, out.Script, out.Amount, seed)
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

	signed := tx.AsLatestVersion()
	for i, _ := range signed.Inputs {
		err := signed.SignInput(raw, i, []common.Address{account})
		if err != nil {
			return err
		}
	}
	fmt.Println(hex.EncodeToString(signed.Marshal()))
	return nil
}

func sendTransactionCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "sendrawtransaction", []interface{}{
		c.String("raw"),
	})
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getRoundByNumberCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getroundbynumber", []interface{}{
		c.String("id"),
		c.Uint64("number"),
	})
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getRoundByHashCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getroundbyhash", []interface{}{
		c.String("hash"),
	})
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func listSnapshotsCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "listsnapshots", []interface{}{
		c.Uint64("since"),
		c.Uint64("count"),
		c.Bool("sig"),
		c.Bool("tx"),
	})
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getTransactionCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "gettransaction", []interface{}{
		c.String("hash"),
	})
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func listMintDistributionsCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "listmintdistributions", []interface{}{
		c.Uint64("since"),
		c.Uint64("count"),
		c.Bool("tx"),
	})
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func getInfoCmd(c *cli.Context) error {
	data, err := callRPC(c.String("node"), "getinfo", []interface{}{})
	if err == nil {
		fmt.Println(string(data))
	}
	return err
}

func setupTestNetCmd(c *cli.Context) error {
	var signers, payees []common.Address

	randomPubAccount := func() common.Address {
		seed := make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			panic(err)
		}
		account := common.NewAddressFromSeed(seed)
		account.PrivateViewKey = account.PublicSpendKey.DeterministicHashDerive()
		account.PublicViewKey = account.PrivateViewKey.Public()
		return account
	}
	for i := 0; i < 7; i++ {
		signers = append(signers, randomPubAccount())
		payees = append(payees, randomPubAccount())
	}

	inputs := make([]map[string]string, 0)
	for i, _ := range signers {
		inputs = append(inputs, map[string]string{
			"signer":  signers[i].String(),
			"payee":   payees[i].String(),
			"balance": "10000",
		})
	}
	genesis := map[string]interface{}{
		"epoch": time.Now().Unix(),
		"nodes": inputs,
		"domains": []map[string]string{
			{
				"signer":  signers[0].String(),
				"balance": "50000",
			},
		},
	}
	genesisData, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(genesisData))

	nodes := make([]map[string]string, 0)
	for i, a := range signers {
		nodes = append(nodes, map[string]string{
			"host":   fmt.Sprintf("127.0.0.1:700%d", i+1),
			"signer": a.String(),
		})
	}
	nodesData, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(nodesData))

	for i, a := range signers {
		dir := fmt.Sprintf("/tmp/mixin-700%d", i+1)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}

		store, err := storage.NewBadgerStore(dir)
		if err != nil {
			return err
		}
		defer store.Close()

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

func callRPC(node, method string, params []interface{}) ([]byte, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	body, err := json.Marshal(map[string]interface{}{
		"method": method,
		"params": params,
	})
	if err != nil {
		panic(err)
	}

	endpoint := "http://" + node
	if strings.HasPrefix(node, "http") {
		endpoint = node
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Close = true
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data  interface{} `json:"data"`
		Error interface{} `json:"error"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("ERROR %s", result.Error)
	}

	return json.Marshal(result.Data)
}

type signerInput struct {
	Inputs []struct {
		Hash    crypto.Hash         `json:"hash"`
		Index   int                 `json:"index"`
		Deposit *common.DepositData `json:"deposit,omitempty"`
		Keys    []crypto.Key        `json:"keys"`
		Mask    crypto.Key          `json:"mask"`
	} `json:"inputs"`
	Outputs []struct {
		Type     uint8            `json:"type"`
		Script   common.Script    `json:"script"`
		Accounts []common.Address `json:"accounts"`
		Amount   common.Integer   `json:"amount"`
	}
	Asset crypto.Hash `json:"asset"`
	Extra string      `json:"extra"`
	Node  string      `json:"-"`
}

func (raw signerInput) ReadUTXO(hash crypto.Hash, index int) (*common.UTXO, error) {
	utxo := &common.UTXO{}

	for _, in := range raw.Inputs {
		if in.Hash == hash && in.Index == index && len(in.Keys) > 0 {
			utxo.Keys = in.Keys
			utxo.Mask = in.Mask
			return utxo, nil
		}
	}

	data, err := callRPC(raw.Node, "gettransaction", []interface{}{hash.String()})
	if err != nil {
		return nil, err
	}
	var tx common.SignedTransaction
	err = json.Unmarshal(data, &tx)
	if err != nil {
		return nil, err
	}
	for i, out := range tx.Outputs {
		if i == index && len(out.Keys) > 0 {
			utxo.Keys = out.Keys
			utxo.Mask = out.Mask
			return utxo, nil
		}
	}

	return nil, fmt.Errorf("invalid input %s#%d", hash.String(), index)
}

func (raw signerInput) CheckDepositInput(deposit *common.DepositData, tx crypto.Hash) error {
	return nil
}

func (raw signerInput) ReadLastMintDistribution(group string) (*common.MintDistribution, error) {
	return nil, nil
}
