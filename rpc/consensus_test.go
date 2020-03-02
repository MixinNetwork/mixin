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
	"sort"
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
	NODES  = 8
	INPUTS = 100
)

func TestConsensus(t *testing.T) {
	assert := assert.New(t)

	root, err := ioutil.TempDir("", "mixin-consensus-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	accounts, payees, gdata, ndata := setupTestNet(root)
	assert.Len(accounts, NODES)

	epoch := time.Unix(1551312000, 0)
	nodes := make([]*Node, 0)
	instances := make([]*kernel.Node, 0)
	stores := make([]storage.Store, 0)
	for i, _ := range accounts {
		dir := fmt.Sprintf("%s/mixin-170%02d", root, i+1)
		config.Initialize(dir + "/config.json")
		cache := fastcache.New(config.Custom.MaxCacheSize * 1024 * 1024)
		store, err := storage.NewBadgerStore(dir)
		assert.Nil(err)
		assert.NotNil(store)
		stores = append(stores, store)
		testIntializeConfig(dir + "/config.json")
		if i == 0 {
			kernel.TestMockDiff(epoch.Sub(time.Now()))
		}
		node, err := kernel.SetupNode(store, cache, fmt.Sprintf(":170%02d", i+1), dir)
		assert.Nil(err)
		assert.NotNil(node)
		instances = append(instances, node)
		host := fmt.Sprintf("127.0.0.1:180%02d", i+1)
		nodes = append(nodes, &Node{Signer: node.Signer, Host: host})
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
	gt := testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(1 * time.Second)))

	domainAddress := accounts[0].String()
	deposits := make([]*common.VersionedTransaction, 0)
	for i := 0; i < INPUTS; i++ {
		raw := fmt.Sprintf(`{"version":1,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"deposit":{"chain":"8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27","asset":"0xa974c709cfb4566686553a20790685a47aceaa33","transaction":"0xc7c1132b58e1f64c263957d7857fe5ec5294fce95d30dcd64efef71da1%06d","index":0,"amount":"100.035"}}],"outputs":[{"type":0,"amount":"100.035","script":"fffe01","accounts":["%s"]}]}`, i, domainAddress)
		mathRand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[mathRand.Intn(len(nodes))].Host, accounts[0], raw)
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
			}(nodes[i].Host, hex.EncodeToString(d.Marshal()))
		}
		wg.Wait()
	}

	time.Sleep(10 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS+NODES+1, tl)
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(1 * time.Second)))

	utxos := make([]*common.VersionedTransaction, 0)
	for _, d := range deposits {
		raw := fmt.Sprintf(`{"version":1,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"hash":"%s","index":0}],"outputs":[{"type":0,"amount":"100.035","script":"fffe01","accounts":["%s"]}]}`, d.PayloadHash().String(), domainAddress)
		mathRand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[mathRand.Intn(len(nodes))].Host, accounts[0], raw)
		assert.Nil(err)
		assert.NotNil(tx)
		if tx != nil {
			utxos = append(utxos, &common.VersionedTransaction{SignedTransaction: *tx})
		}
	}
	assert.Equal(len(utxos), INPUTS)

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
			}(nodes[i].Host, hex.EncodeToString(tx.Marshal()))
		}
		wg.Wait()
	}

	time.Sleep(10 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1, tl)
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(31 * time.Second)))

	kernel.TestMockDiff((config.KernelMintTimeBegin + 24) * time.Hour)
	time.Sleep(3 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1, tl)
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(31 * time.Second)))

	input, err := testBuildPledgeInput(assert, nodes[0].Host, accounts[0], utxos)
	assert.Nil(err)
	time.Sleep(3 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1, tl)
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(61 * time.Second)))

	pn := testPledgeNewNode(assert, nodes[0].Host, accounts[0], gdata, ndata, input, root)
	time.Sleep(3 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2, tl)
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.After(epoch.Add((config.KernelMintTimeBegin + 24) * time.Hour)))
	assert.Equal("499876.71232883", gt.PoolSize.String())

	all := testListNodes(nodes[0].Host)
	assert.Len(all, NODES+1)
	assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
	assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
	assert.Equal("PLEDGING", all[NODES].State)

	kernel.TestMockDiff(11 * time.Hour)
	time.Sleep(3 * time.Second)
	all = testListNodes(nodes[0].Host)
	assert.Len(all, NODES+1)
	assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
	assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
	assert.Equal("PLEDGING", all[NODES].State)

	kernel.TestMockDiff(1 * time.Hour)
	time.Sleep(5 * time.Second)
	all = testListNodes(nodes[0].Host)
	assert.Len(all, NODES+1)
	assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
	assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
	assert.Equal("ACCEPTED", all[NODES].State)

	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1, tl)
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.After(epoch.Add((config.KernelMintTimeBegin + 24) * time.Hour)))
	assert.Equal("499876.71232883", gt.PoolSize.String())

	kernel.TestMockDiff(364 * 24 * time.Hour)
	time.Sleep(3 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1, tl)

	input = testSendDummyTransaction(assert, nodes[0].Host, accounts[0], input, "3.5")
	time.Sleep(5 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1+1, tl)
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
		assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
		assert.Equal("ACCEPTED", all[NODES].State)
	}

	signer, payee := testGetNodeToRemove(instances[0].NetworkId(), accounts, payees)
	input = testSendDummyTransaction(assert, nodes[0].Host, accounts[0], input, "3.5")
	nodes = testRemoveNode(nodes, signer)
	time.Sleep(5 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1+1+2, tl)
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), signer.String())
		assert.Equal(all[NODES].Payee.String(), payee.String())
		assert.Equal("REMOVED", all[NODES].State)
	}

	input = testSendDummyTransaction(assert, nodes[0].Host, payee, all[NODES].Transaction.String(), "10000")
	time.Sleep(10 * time.Second)
	tl, sl = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1+1+2+1, tl)
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), signer.String())
		assert.Equal(all[NODES].Payee.String(), payee.String())
		assert.Equal("REMOVED", all[NODES].State)
	}
}

func testRemoveNode(nodes []*Node, r common.Address) []*Node {
	tmp := []*Node{{Host: "127.0.0.1:18099"}}
	for _, n := range nodes {
		if n.Signer.String() != r.String() {
			tmp = append(tmp, n)
		}
	}
	mathRand.Seed(time.Now().UnixNano())
	for n := len(tmp); n > 0; n-- {
		randIndex := mathRand.Intn(n)
		tmp[n-1], tmp[randIndex] = tmp[randIndex], tmp[n-1]
	}
	return tmp
}

func testIntializeConfig(file string) {
	f, _ := ioutil.ReadFile(file)
	var c struct {
		Environment    string        `json:"environment"`
		Signer         crypto.Key    `json:"signer"`
		Listener       string        `json:"listener"`
		MaxCacheSize   int           `json:"max-cache-size"`
		ElectionTicker int           `json:"election-ticker"`
		CacheTTL       time.Duration `json:"cache-ttl"`
	}
	json.Unmarshal(f, &c)
	if c.CacheTTL == 0 {
		c.CacheTTL = 3600
	}
	if c.MaxCacheSize == 0 {
		c.MaxCacheSize = 32
	}
	if c.ElectionTicker == 0 {
		c.ElectionTicker = 2
	}
	config.Custom.Environment = c.Environment
	config.Custom.Signer = c.Signer
	config.Custom.Listener = c.Listener
	config.Custom.CacheTTL = c.CacheTTL
	config.Custom.MaxCacheSize = c.MaxCacheSize
	config.Custom.ElectionTicker = c.ElectionTicker
}

func testSendDummyTransaction(assert *assert.Assertions, node string, domain common.Address, th, amount string) string {
	raw, err := json.Marshal(map[string]interface{}{
		"version": 1,
		"asset":   "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
		"inputs": []map[string]interface{}{{
			"hash":  th,
			"index": 0,
		}},
		"outputs": []map[string]interface{}{{
			"type":     0,
			"amount":   amount,
			"script":   "fffe01",
			"accounts": []string{domain.String()},
		}},
	})
	tx, err := testSignTransaction(node, domain, string(raw))
	assert.Nil(err)
	ver := common.VersionedTransaction{SignedTransaction: *tx}
	input, err := testSendTransaction(node, hex.EncodeToString(ver.Marshal()))
	assert.Nil(err)
	var hash map[string]string
	err = json.Unmarshal([]byte(input), &hash)
	assert.Nil(err)
	return hash["hash"]
}

func testPledgeNewNode(assert *assert.Assertions, node string, domain common.Address, genesisData, nodesData []byte, input, root string) Node {
	var signer, payee common.Address

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
	signer = randomPubAccount()
	payee = randomPubAccount()

	dir := fmt.Sprintf("%s/mixin-17099", root)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		panic(err)
	}

	configData, err := json.MarshalIndent(map[string]interface{}{
		"environment":     "test",
		"signer":          signer.PrivateSpendKey.String(),
		"listener":        "127.0.0.1:17099",
		"cache-ttl":       3600,
		"election-ticker": 2,
		"max-cache-size":  128,
	}, "", "  ")
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(dir+"/config.json", configData, 0644)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(dir+"/genesis.json", genesisData, 0644)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(dir+"/nodes.json", nodesData, 0644)
	if err != nil {
		panic(err)
	}

	raw, err := json.Marshal(map[string]interface{}{
		"version": 1,
		"asset":   "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
		"inputs": []map[string]interface{}{{
			"hash":  input,
			"index": 1,
		}},
		"outputs": []map[string]interface{}{{
			"type":   common.OutputTypeNodePledge,
			"amount": "10000",
		}},
		"extra": signer.PublicSpendKey.String() + payee.PublicSpendKey.String(),
	})
	tx, err := testSignTransaction(node, domain, string(raw))
	assert.Nil(err)
	ver := common.VersionedTransaction{SignedTransaction: *tx}
	_, err = testSendTransaction(node, hex.EncodeToString(ver.Marshal()))
	assert.Nil(err)

	config.Initialize(dir + "/config.json")
	cache := fastcache.New(config.Custom.MaxCacheSize * 1024 * 1024)
	store, err := storage.NewBadgerStore(dir)
	assert.Nil(err)
	assert.NotNil(store)
	testIntializeConfig(dir + "/config.json")
	pnode, err := kernel.SetupNode(store, cache, fmt.Sprintf(":170%02d", 99), dir)
	assert.Nil(err)
	assert.NotNil(pnode)
	go pnode.Loop()
	go StartHTTP(store, pnode, 18099)

	return Node{Signer: signer, Payee: payee}
}

func testBuildPledgeInput(assert *assert.Assertions, node string, domain common.Address, utxos []*common.VersionedTransaction) (string, error) {
	inputs := []map[string]interface{}{}
	for _, tx := range utxos {
		inputs = append(inputs, map[string]interface{}{
			"hash":  tx.PayloadHash().String(),
			"index": 0,
		})
	}
	raw, err := json.Marshal(map[string]interface{}{
		"version": 1,
		"asset":   "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
		"inputs":  inputs,
		"outputs": []map[string]interface{}{{
			"type":     0,
			"amount":   "3.5",
			"script":   "fffe01",
			"accounts": []string{domain.String()},
		}, {
			"type":     0,
			"amount":   "10000",
			"script":   "fffe01",
			"accounts": []string{domain.String()},
		}},
	})
	tx, err := testSignTransaction(node, domain, string(raw))
	assert.Nil(err)
	ver := common.VersionedTransaction{SignedTransaction: *tx}
	input, err := testSendTransaction(node, hex.EncodeToString(ver.Marshal()))
	assert.Nil(err)
	var hash map[string]string
	err = json.Unmarshal([]byte(input), &hash)
	return hash["hash"], err
}

func testSendTransaction(node, raw string) (string, error) {
	data, err := callRPC(node, "sendrawtransaction", []interface{}{
		raw,
	})
	return string(data), err
}

func testGetNodeToRemove(networkId crypto.Hash, signers, payees []common.Address) (common.Address, common.Address) {
	nodes := make([][2]common.Address, len(signers))
	for i := range signers {
		nodes[i] = [2]common.Address{signers[i], payees[i]}
	}
	sort.Slice(nodes, func(i, j int) bool {
		a := nodes[i][0].Hash().ForNetwork(networkId)
		b := nodes[j][0].Hash().ForNetwork(networkId)
		return a.String() < b.String()
	})
	return nodes[0][0], nodes[0][1]
}

func setupTestNet(root string) ([]common.Address, []common.Address, []byte, []byte) {
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
		"epoch": 1551312000,
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
		panic(err)
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
		panic(err)
	}

	for i, a := range signers {
		dir := fmt.Sprintf("%s/mixin-170%02d", root, i+1)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}

		configData, err := json.MarshalIndent(map[string]interface{}{
			"environment":     "test",
			"signer":          a.PrivateSpendKey.String(),
			"listener":        nodes[i]["host"],
			"cache-ttl":       3600,
			"election-ticker": 2,
			"max-cache-size":  128,
		}, "", "  ")
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile(dir+"/config.json", configData, 0644)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(dir+"/genesis.json", genesisData, 0644)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(dir+"/nodes.json", nodesData, 0644)
		if err != nil {
			panic(err)
		}
	}
	return signers, payees, genesisData, nodesData
}

func testSignTransaction(node string, account common.Address, rawStr string) (*common.SignedTransaction, error) {
	var raw signerInput
	err := json.Unmarshal([]byte(rawStr), &raw)
	if err != nil {
		panic(err)
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
		if out.Mask.HasValue() {
			tx.Outputs = append(tx.Outputs, &common.Output{
				Type:   out.Type,
				Amount: out.Amount,
				Keys:   out.Keys,
				Script: out.Script,
				Mask:   out.Mask,
			})
		} else {
			seed := make([]byte, 64)
			_, err := rand.Read(seed)
			if err != nil {
				panic(err)
			}
			hash := crypto.NewHash(seed)
			seed = append(hash[:], hash[:]...)
			tx.AddOutputWithType(out.Type, out.Accounts, out.Script, out.Amount, seed)
		}
	}

	extra, err := hex.DecodeString(raw.Extra)
	if err != nil {
		panic(err)
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

func testVerifyInfo(assert *assert.Assertions, nodes []*Node) Info {
	info := testGetGraphInfo(nodes[0].Host)
	for _, n := range nodes {
		a := testGetGraphInfo(n.Host)
		assert.Equal(info.Timestamp, a.Timestamp)
		assert.Equal(info.PoolSize, a.PoolSize)
	}
	return info
}

func testVerifySnapshots(assert *assert.Assertions, nodes []*Node) (int, int) {
	filters := make([]map[string]*common.Snapshot, 0)
	for _, n := range nodes {
		filters = append(filters, testListSnapshots(n.Host))
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

type Node struct {
	Signer      common.Address `json:"signer"`
	Payee       common.Address `json:"payee"`
	State       string         `json:"state"`
	Transaction crypto.Hash    `json:"transaction"`
	Host        string         `json:"-"`
}

func testListNodes(node string) []*Node {
	data, err := callRPC(node, "listallnodes", []interface{}{})
	if err != nil {
		panic(err)
	}
	var nodes []*Node
	err = json.Unmarshal(data, &nodes)
	if err != nil {
		panic(err)
	}
	return nodes
}

type Info struct {
	Timestamp time.Time
	PoolSize  common.Integer
}

func testGetGraphInfo(node string) Info {
	data, err := callRPC(node, "getinfo", []interface{}{})
	if err != nil {
		panic(err)
	}
	var info struct {
		Timestamp string `json:"timestamp"`
		Mint      struct {
			PoolSize common.Integer `json:"pool"`
		} `json:"mint"`
	}
	err = json.Unmarshal(data, &info)
	if err != nil {
		panic(err)
	}
	t, err := time.Parse(time.RFC3339Nano, info.Timestamp)
	if err != nil {
		panic(err)
	}
	return Info{
		Timestamp: t,
		PoolSize:  info.Mint.PoolSize,
	}
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
		Mask     crypto.Key       `json:"mask"`
		Keys     []crypto.Key     `json:"keys"`
		Amount   common.Integer   `json:"amount"`
		Script   common.Script    `json:"script"`
		Accounts []common.Address `json:"accounts"`
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
