package rpc

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	mathRand "math/rand"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/VictoriaMetrics/fastcache"
	"github.com/stretchr/testify/assert"
)

const (
	NODES  = 7
	INPUTS = 100
)

func TestConsensus(t *testing.T) {
	assert := assert.New(t)

	root, err := ioutil.TempDir("", "mixin-consensus-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	accounts, err := setupTestNet(root)
	assert.Nil(err)
	assert.Len(accounts, NODES)

	nodes := make([]string, 0)
	instances := make([]*kernel.Node, 0)
	stores := make([]storage.Store, 0)
	for i, _ := range accounts {
		dir := fmt.Sprintf("%s/mixin-170%02d", root, i+1)
		config.Initialize(dir + "/config.json")
		cache := fastcache.New(config.Custom.MaxCacheSize * 1024 * 1024)
		store, err := storage.NewBadgerStore(dir, cache)
		assert.Nil(err)
		assert.NotNil(store)
		stores = append(stores, store)
		testIntializeConfig(dir + "/config.json")
		node, err := kernel.SetupNode(store, cache, fmt.Sprintf(":170%02d", i+1), dir)
		assert.Nil(err)
		assert.NotNil(node)
		instances = append(instances, node)
		host := fmt.Sprintf("127.0.0.1:180%02d", i+1)
		nodes = append(nodes, host)
	}
	for i, n := range instances {
		go func(node *kernel.Node, store storage.Store, num int) {
			go StartHTTP(store, node, 18000+num+1)
			go node.Loop()
		}(n, stores[i], i)
	}
	time.Sleep(5 * time.Second)

	tl, sl := testVerifySnapshots(assert, nodes)
	assert.Equal(NODES+1, tl)
	assert.Equal(NODES+1, sl)

	domainAddress := accounts[0].String()
	deposits := make([]*common.VersionedTransaction, 0)
	for i := 0; i < INPUTS; i++ {
		raw := fmt.Sprintf(`{"version":1,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"deposit":{"chain":"8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27","asset":"0xa974c709cfb4566686553a20790685a47aceaa33","transaction":"0xc7c1132b58e1f64c263957d7857fe5ec5294fce95d30dcd64efef71da1%06d","index":0,"amount":"100.035"}}],"outputs":[{"type":0,"amount":"100.035","script":"fffe01","accounts":["%s"]}]}`, i, domainAddress)
		mathRand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[mathRand.Intn(len(nodes))], accounts[0], raw)
		assert.Nil(err)
		assert.NotNil(tx)
		deposits = append(deposits, &common.VersionedTransaction{SignedTransaction: *tx})
	}

	for _, d := range deposits {
		mathRand.Seed(time.Now().UnixNano())
		for n := len(nodes); n > 0; n-- {
			randIndex := mathRand.Intn(n)
			nodes[n-1], nodes[randIndex] = nodes[randIndex], nodes[n-1]
		}
		wg := &sync.WaitGroup{}
		for i := mathRand.Intn(len(nodes) - 1); i < len(nodes); i++ {
			wg.Add(1)
			go func(n string, raw string) {
				defer wg.Done()
				id, err := testSendTransaction(n, raw)
				assert.Nil(err)
				assert.Len(id, 75)
			}(nodes[i], hex.EncodeToString(d.Marshal()))
		}
		wg.Wait()
	}

	time.Sleep(10 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS+NODES+1, tl)

	utxos := make([]*common.VersionedTransaction, 0)
	for _, d := range deposits {
		raw := fmt.Sprintf(`{"version":1,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"hash":"%s","index":0}],"outputs":[{"type":0,"amount":"100.035","script":"fffe01","accounts":["%s"]}]}`, d.PayloadHash().String(), domainAddress)
		mathRand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[mathRand.Intn(len(nodes))], accounts[0], raw)
		assert.Nil(err)
		assert.NotNil(tx)
		utxos = append(utxos, &common.VersionedTransaction{SignedTransaction: *tx})
	}

	for _, tx := range utxos {
		mathRand.Seed(time.Now().UnixNano())
		for n := len(nodes); n > 0; n-- {
			randIndex := mathRand.Intn(n)
			nodes[n-1], nodes[randIndex] = nodes[randIndex], nodes[n-1]
		}
		wg := &sync.WaitGroup{}
		for i := mathRand.Intn(len(nodes) - 1); i < len(nodes); i++ {
			wg.Add(1)
			go func(n string, raw string) {
				defer wg.Done()
				id, err := testSendTransaction(n, raw)
				assert.Nil(err)
				assert.Len(id, 75)
			}(nodes[i], hex.EncodeToString(tx.Marshal()))
		}
		wg.Wait()
	}

	time.Sleep(10 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1, tl)
}

func testIntializeConfig(file string) {
	f, _ := ioutil.ReadFile(file)
	var c struct {
		Signer       crypto.Key    `json:"signer"`
		Listener     string        `json:"listener"`
		MaxCacheSize int           `json:"max-cache-size"`
		CacheTTL     time.Duration `json:"cache-ttl"`
	}
	json.Unmarshal(f, &c)
	if c.CacheTTL == 0 {
		c.CacheTTL = 3600
	}
	if c.MaxCacheSize == 0 {
		c.MaxCacheSize = 32
	}
	config.Custom.Signer = c.Signer
	config.Custom.Listener = c.Listener
	config.Custom.CacheTTL = c.CacheTTL
	config.Custom.MaxCacheSize = c.MaxCacheSize
}

func testSendTransaction(node, raw string) (string, error) {
	data, err := callRPC(node, "sendrawtransaction", []interface{}{
		raw,
	})
	return string(data), err
}

func setupTestNet(root string) ([]common.Address, error) {
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
	for i := 0; i < NODES; i++ {
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
		return nil, err
	}

	nodes := make([]map[string]string, 0)
	for i, a := range signers {
		nodes = append(nodes, map[string]string{
			"host":   fmt.Sprintf("127.0.0.1:170%02d", i+1),
			"signer": a.String(),
		})
	}
	nodesData, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return nil, err
	}

	for i, a := range signers {
		dir := fmt.Sprintf("%s/mixin-170%02d", root, i+1)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, err
		}

		configData, err := json.MarshalIndent(map[string]interface{}{
			"signer":         a.PrivateSpendKey.String(),
			"listener":       nodes[i]["host"],
			"cache-ttl":      3600,
			"max-cache-size": 128,
		}, "", "  ")
		if err != nil {
			return nil, err
		}

		err = ioutil.WriteFile(dir+"/config.json", configData, 0644)
		if err != nil {
			return nil, err
		}
		err = ioutil.WriteFile(dir+"/genesis.json", genesisData, 0644)
		if err != nil {
			return nil, err
		}
		err = ioutil.WriteFile(dir+"/nodes.json", nodesData, 0644)
		if err != nil {
			return nil, err
		}
	}
	return signers, nil
}

func testSignTransaction(node string, account common.Address, rawStr string) (*common.SignedTransaction, error) {
	var raw signerInput
	err := json.Unmarshal([]byte(rawStr), &raw)
	if err != nil {
		return nil, err
	}
	raw.Node = node

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
			return nil, fmt.Errorf("invalid output type %d", out.Type)
		}
		tx.AddRandomScriptOutput(out.Accounts, out.Script, out.Amount)
	}

	extra, err := hex.DecodeString(raw.Extra)
	if err != nil {
		return nil, err
	}
	tx.Extra = extra

	signed := &common.SignedTransaction{Transaction: *tx}
	for i, _ := range signed.Inputs {
		err := signed.SignInput(raw, i, []common.Address{account})
		if err != nil {
			return nil, err
		}
	}
	return signed, nil
}

func testVerifySnapshots(assert *assert.Assertions, nodes []string) (int, int) {
	filters := make([]map[string]*common.Snapshot, 0)
	for _, n := range nodes {
		filters = append(filters, testListSnapshots(n))
	}
	t, s := make(map[string]bool), make(map[string]bool)
	for i := 0; i < len(filters)-1; i++ {
		a, b := filters[i], filters[i+1]
		m, n := make(map[string]bool), make(map[string]bool)
		for k, _ := range a {
			s[k] = true
			assert.NotNil(a[k])
			assert.NotNil(b[k])
			if a[k] != nil && b[k] != nil {
				assert.Equal(b[k].Transaction, a[k].Transaction)
			}
			if a[k] != nil {
				m[a[k].Transaction.String()] = true
				t[a[k].Transaction.String()] = true
			}
		}
		for k, _ := range b {
			s[k] = true
			assert.NotNil(a[k])
			assert.NotNil(b[k])
			if a[k] != nil && b[k] != nil {
				assert.Equal(b[k].Transaction, a[k].Transaction)
			}
			if b[k] != nil {
				n[b[k].Transaction.String()] = true
				t[b[k].Transaction.String()] = true
			}
		}
		assert.Equal(len(m), len(n))
		assert.Equal(len(filters[i]), len(filters[i+1]))
	}
	return len(t), len(s)
}

func testListSnapshots(node string) map[string]*common.Snapshot {
	data, err := callRPC(node, "listsnapshots", []interface{}{
		0,
		100000,
		false,
		false,
	})

	var snapshots []*common.Snapshot
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(data, &snapshots)
	if err != nil {
		panic(err)
	}
	filter := make(map[string]*common.Snapshot)
	for _, s := range snapshots {
		filter[s.Hash.String()] = s
	}
	return filter
}

var httpClient *http.Client

func callRPC(node, method string, params []interface{}) ([]byte, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
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

func (raw signerInput) ReadUTXO(hash crypto.Hash, index int) (*common.UTXOWithLock, error) {
	utxo := &common.UTXOWithLock{}

	for _, in := range raw.Inputs {
		if in.Hash == hash && in.Index == index && len(in.Keys) > 0 {
			utxo.Keys = in.Keys
			utxo.Mask = in.Mask
			return utxo, nil
		}
	}

	data, err := callRPC(raw.Node, "getutxo", []interface{}{hash.String(), index})
	if err != nil {
		return nil, err
	}
	var out common.UTXOWithLock
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}
	if out.Amount.Sign() == 0 {
		return nil, fmt.Errorf("invalid input %s#%d", hash.String(), index)
	}
	utxo.Keys = out.Keys
	utxo.Mask = out.Mask
	return utxo, nil
}

func (raw signerInput) CheckDepositInput(deposit *common.DepositData, tx crypto.Hash) error {
	return nil
}

func (raw signerInput) ReadLastMintDistribution(group string) (*common.MintDistribution, error) {
	return nil, nil
}
