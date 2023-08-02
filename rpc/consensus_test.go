package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/assert"
)

var (
	NODES  = 8
	INPUTS = 100
)

func TestConsensus(t *testing.T) {
	testConsensus(t, 0)
}

func testConsensus(t *testing.T, snapVersionMint int) {
	assert := assert.New(t)
	kernel.TestMockReset()

	level, _ := strconv.ParseInt(os.Getenv("LOG"), 10, 64)
	enableElection, err := strconv.ParseBool(os.Getenv("ELECTION"))
	if err != nil {
		enableElection = true
	}
	inputs, _ := strconv.ParseInt(os.Getenv("INPUT"), 10, 64)
	logger.SetLevel(int(level))
	if inputs > 0 {
		INPUTS = int(inputs)
	}
	t.Logf("TEST WITH %d INPUTS AT %s\n", INPUTS, time.Now())

	root, err := os.MkdirTemp("", "mixin-consensus-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	accounts, payees, gdata, plist := setupTestNet(root)
	assert.Len(accounts, NODES)

	epoch := time.Unix(1551312000, 0)
	nodes := make([]*Node, 0)
	instances := make([]*kernel.Node, 0)
	for i := range accounts {
		dir := fmt.Sprintf("%s/mixin-170%02d", root, i+1)
		custom, err := config.Initialize(dir + "/config.toml")
		assert.Nil(err)
		cache := newCache(custom)
		store, err := storage.NewBadgerStore(custom, dir)
		assert.Nil(err)
		assert.NotNil(store)
		if i == 0 {
			kernel.TestMockDiff(epoch.Sub(time.Now()))
		}
		node, err := kernel.SetupNode(custom, store, cache, fmt.Sprintf(":170%02d", i+1), dir)
		assert.Nil(err)
		assert.NotNil(node)
		err = node.PingNeighborsFromConfig()
		assert.Nil(err)
		instances = append(instances, node)
		host := fmt.Sprintf("127.0.0.1:180%02d", i+1)
		nodes = append(nodes, &Node{Signer: node.Signer, Host: host})
		t.Logf("NODES#%d %s %s\n", i, node.IdForNetwork, host)

		server := NewServer(custom, store, node, 18000+i+1)
		defer server.Close()
		go func(node *kernel.Node, store storage.Store, num int, s *http.Server) {
			go s.ListenAndServe()
			go node.Loop()
		}(node, store, i, server)
	}
	defer func() {
		var wg sync.WaitGroup
		for _, n := range instances {
			wg.Add(1)
			go func(node *kernel.Node) {
				node.Teardown()
				wg.Done()
			}(n)
		}
		wg.Wait()
	}()
	time.Sleep(3 * time.Second)

	transactionsCount := NODES + 1
	tl, sl := testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))
	assert.Equal(transactionsCount, len(sl))
	gt := testVerifyInfo(assert, nodes)
	assert.Truef(gt.Timestamp.Before(epoch.Add(1*time.Second)), "%s should before %s", gt.Timestamp, epoch.Add(1*time.Second))

	genesisAmount := 10003.5 / float64(INPUTS)
	domainAddress := accounts[0].String()
	deposits := make([]*common.VersionedTransaction, 0)
	for i := 0; i < INPUTS; i++ {
		raw := fmt.Sprintf(`{"version":2,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"deposit":{"chain":"8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27","asset":"0xa974c709cfb4566686553a20790685a47aceaa33","transaction":"0xc7c1132b58e1f64c263957d7857fe5ec5294fce95d30dcd64efef71da1%06d","index":0,"amount":"%f"}}],"outputs":[{"type":0,"amount":"%f","script":"fffe01","accounts":["%s"]}]}`, i, genesisAmount, genesisAmount, domainAddress)
		rand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[rand.Intn(len(nodes))].Host, accounts[0], raw, snapVersionMint)
		assert.Nil(err)
		assert.NotNil(tx)
		deposits = append(deposits, &common.VersionedTransaction{SignedTransaction: *tx})
	}

	testSendTransactionsToNodesWithRetry(t, nodes, deposits[:INPUTS/2])
	testSendTransactionsToNodesWithRetry(t, nodes, deposits[INPUTS/2:])
	transactionsCount = transactionsCount + INPUTS
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))

	gt = testVerifyInfo(assert, nodes)
	assert.Truef(gt.Timestamp.Before(epoch.Add(7*time.Second)), "%s should before %s", gt.Timestamp, epoch.Add(7*time.Second))
	hr := testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	assert.NotNil(hr)
	assert.GreaterOrEqual(hr.Round, uint64(0))
	t.Logf("DEPOSIT TEST DONE AT %s\n", time.Now())

	utxos := make([]*common.VersionedTransaction, 0)
	for _, d := range deposits {
		raw := fmt.Sprintf(`{"version":2,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"hash":"%s","index":0}],"outputs":[{"type":0,"amount":"%f","script":"fffe01","accounts":["%s"]}]}`, d.PayloadHash().String(), genesisAmount, domainAddress)
		rand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[rand.Intn(len(nodes))].Host, accounts[0], raw, snapVersionMint)
		assert.Nil(err)
		assert.NotNil(tx)
		if tx != nil {
			utxos = append(utxos, &common.VersionedTransaction{SignedTransaction: *tx})
		}
	}
	assert.Equal(INPUTS, len(utxos))

	testSendTransactionsToNodesWithRetry(t, nodes, utxos[:INPUTS/2])
	testSendTransactionsToNodesWithRetry(t, nodes, utxos[INPUTS/2:])
	transactionsCount = transactionsCount + INPUTS
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))

	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(31 * time.Second)))
	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	assert.NotNil(hr)
	assert.Greater(hr.Round, uint64(0))
	t.Logf("INPUT TEST DONE AT %s\n", time.Now())

	if !enableElection {
		return
	}

	all := testListNodes(nodes[0].Host)
	assert.Len(all, NODES)
	assert.Equal("ACCEPTED", all[NODES-1].State)

	input, _ := testBuildPledgeInput(t, nodes, accounts[0], utxos, snapVersionMint)
	time.Sleep(3 * time.Second)
	transactionsCount = transactionsCount + 1
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(61 * time.Second)))
	t.Logf("PLEDGE %s\n", input)

	dummyAmount := common.NewIntegerFromString("3.5").Div(NODES).String()
	dummyInputs := make([]*common.Input, NODES)
	for i := range dummyInputs {
		hash, _ := crypto.HashFromString(input)
		dummyInputs[i] = &common.Input{Hash: hash, Index: i}
	}

	for i := 0; i < 3; i++ {
		dummyInputs = testSendDummyTransactionsWithRetry(t, nodes, accounts[0], dummyInputs, dummyAmount, snapVersionMint)
		transactionsCount = transactionsCount + len(dummyInputs)
	}

	mints := testListMintDistributions(nodes[0].Host)
	assert.Len(mints, 0)

	kernel.TestMockDiff((config.KernelMintTimeBegin + 24) * time.Hour)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(31 * time.Second)))

	pn, pi, sv := testPledgeNewNode(t, nodes, accounts[0], gdata, plist, input, root, snapVersionMint)
	t.Logf("PLEDGE %s\n", pn.Signer)
	transactionsCount = transactionsCount + 1
	defer pi.Teardown()
	defer sv.Close()

	for i := 0; i < 5; i++ {
		dummyInputs = testSendDummyTransactionsWithRetry(t, nodes, accounts[0], dummyInputs, dummyAmount, snapVersionMint)
		transactionsCount = transactionsCount + len(dummyInputs)
	}

	mints = testListMintDistributions(nodes[0].Host)
	assert.Len(mints, 1)
	tx := mints[0]
	assert.Len(tx.Inputs, 1)
	mint := tx.Inputs[0].Mint
	daily := common.NewIntegerFromString("136.98630136")
	assert.Equal("UNIVERSAL", mint.Group)
	assert.Equal(uint64(1), mint.Batch)
	assert.Equal(daily, mint.Amount)
	assert.Len(tx.Outputs, NODES+2)
	total := common.Zero
	for i, o := range tx.Outputs {
		if i < NODES {
			total = total.Add(o.Amount)
			assert.Equal("fffe01", o.Script.String())
			assert.Equal(uint8(common.OutputTypeScript), o.Type)
			assert.Len(o.Keys, 1)
		} else if i == NODES {
			assert.Equal("fffe01", o.Script.String())
			assert.Equal(daily.Div(10).Mul(4), o.Amount)
			assert.Equal(uint8(common.OutputTypeScript), o.Type)
			assert.Len(o.Keys, 1)
		} else if i == NODES+1 {
			custodian := daily.Div(10).Mul(4)
			total = total.Add(custodian)
			light := daily.Sub(total)
			assert.Equal("fffe40", o.Script.String())
			assert.Equal(light, o.Amount)
			assert.Equal(uint8(common.OutputTypeScript), o.Type)
			assert.Len(o.Keys, 1)
		}
	}

	transactionsCount = transactionsCount + 1
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.After(epoch.Add((config.KernelMintTimeBegin + 24) * time.Hour)))
	assert.Equal("499863.01369864", gt.PoolSize.String())
	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	assert.NotNil(hr)
	assert.Greater(hr.Round, uint64(0))
	hr = testDumpGraphHead(nodes[0].Host, pi.IdForNetwork)
	assert.Nil(hr)

	mints = testListMintDistributions(nodes[0].Host)
	assert.Len(mints, 1)

	all = testListNodes(nodes[0].Host)
	assert.Len(all, NODES+1)
	assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
	assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
	assert.Equal("PLEDGING", all[NODES].State)
	t.Logf("PLEDGE TEST DONE AT %s\n", time.Now())

	kernel.TestMockDiff(11 * time.Hour)
	time.Sleep(3 * time.Second)
	all = testListNodes(nodes[0].Host)
	assert.Len(all, NODES+1)
	assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
	assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
	assert.Equal("PLEDGING", all[NODES].State)
	hr = testDumpGraphHead(nodes[0].Host, pi.IdForNetwork)
	assert.Nil(hr)

	kernel.TestMockDiff(1 * time.Hour)
	time.Sleep(5 * time.Second)
	all = testListNodes(nodes[0].Host)
	assert.Len(all, NODES+1)
	assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
	assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
	assert.Equal("ACCEPTED", all[NODES].State)
	assert.Equal(len(testListSnapshots(nodes[NODES-1].Host)), len(testListSnapshots(pn.Host)))
	assert.Equal(len(testListSnapshots(nodes[0].Host)), len(testListSnapshots(pn.Host)))
	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	assert.NotNil(hr)
	assert.Greater(hr.Round, uint64(0))
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, instances[0].IdForNetwork)
	assert.NotNil(hr)
	assert.Greater(hr.Round, uint64(0))
	hr = testDumpGraphHead(nodes[0].Host, pi.IdForNetwork)
	assert.NotNil(hr)
	assert.Equal(uint64(0), hr.Round)
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, pi.IdForNetwork)
	assert.NotNil(hr)
	assert.Equal(uint64(0), hr.Round)

	transactionsCount = transactionsCount + 1
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.After(epoch.Add((config.KernelMintTimeBegin + 24) * time.Hour)))
	assert.Equal("499863.01369864", gt.PoolSize.String())
	t.Logf("ACCEPT TEST DONE AT %s\n", time.Now())

	kernel.TestMockDiff(24 * time.Hour)
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
		assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
		assert.Equal("ACCEPTED", all[NODES].State)
	}

	nodes = append(nodes, &Node{Host: "127.0.0.1:18099"})
	signer, payee := testGetNodeToRemove(instances[0].NetworkId(), accounts, payees, 0)
	assert.Equal("XINGmuYCB65rzMgUf1W35pbhj4C7fY9JrzWCL5vGRdL84SPcWVPhtBJ7DAarc1QPt564JwbEdNCH8359kdPRH1ieSM9f96RZ", signer.String())
	assert.Equal("XINMeKsKkSJJCgLWKvakEHaXBNPGfF7RmBu9jx5VZLE6UTuEaW4wSEqVybkH4xhQcqkT5jdiguiN3B3NKt8QBZTUbqZXJ1Fq", payee.String())
	nodes = testRemoveNode(nodes, signer)
	for i := 0; i < 3; i++ {
		dummyInputs = testSendDummyTransactionsWithRetry(t, nodes, accounts[0], dummyInputs, dummyAmount, snapVersionMint)
		transactionsCount = transactionsCount + len(dummyInputs)
	}
	transactionsCount = transactionsCount + 1

	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), signer.String())
		assert.Equal(all[NODES].Payee.String(), payee.String())
		assert.Equal("REMOVED", all[NODES].State)
	}

	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	assert.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, instances[0].IdForNetwork)
	assert.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[0].Host, pi.IdForNetwork)
	assert.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, pi.IdForNetwork)
	assert.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[0].Host, signer.Hash().ForNetwork(instances[0].NetworkId()))
	assert.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, signer.Hash().ForNetwork(instances[0].NetworkId()))
	assert.Greater(hr.Round, uint64(1))

	removalInputs := []*common.Input{{Hash: all[NODES].Transaction, Index: 0}}
	removalInputs = testSendDummyTransactionsWithRetry(t, nodes[:1], payee, removalInputs, "10000", snapVersionMint)
	transactionsCount = transactionsCount + 1
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(transactionsCount, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), signer.String())
		assert.Equal(all[NODES].Payee.String(), payee.String())
		assert.Equal("REMOVED", all[NODES].State)
	}
	t.Logf("REMOVE TEST DONE AT %s\n", time.Now())

	for _, node := range instances {
		t.Log(node.IdForNetwork, node.Peer.Metric())
	}
}

func testRemoveNode(nodes []*Node, r common.Address) []*Node {
	var tmp []*Node
	for _, n := range nodes {
		if n.Signer.String() != r.String() {
			tmp = append(tmp, n)
		}
	}
	rand.Seed(time.Now().UnixNano())
	for n := len(tmp); n > 0; n-- {
		randIndex := rand.Intn(n)
		tmp[n-1], tmp[randIndex] = tmp[randIndex], tmp[n-1]
	}
	return tmp
}

func testSendDummyTransactionsWithRetry(t *testing.T, nodes []*Node, domain common.Address, inputs []*common.Input, amount string, snapVersionMint int) []*common.Input {
	outputs := testSendDummyTransactions(nodes, domain, inputs, amount, snapVersionMint)
	time.Sleep(3 * time.Second)

	var missingInputs []*common.Input
	var missingNodes []*Node
	for i, in := range outputs {
		data, _ := callRPC(nodes[i].Host, "gettransaction", []any{in.Hash.String()})
		var res map[string]string
		json.Unmarshal([]byte(data), &res)
		hash, _ := crypto.HashFromString(res["snapshot"])
		if hash.HasValue() {
			continue
		}
		t.Logf("DUMMY UTXO %s PENDING IN %s AT %s\n", inputs[i].Hash, nodes[i].Host, time.Now())
		hash, _ = crypto.HashFromString(res["hash"])
		if !hash.HasValue() {
			t.Logf("DUMMY UTXO %s MISSING IN %s AT %s\n", inputs[i].Hash, nodes[i].Host, time.Now())
		}
		missingInputs = append(missingInputs, inputs[i])
		missingNodes = append(missingNodes, nodes[i])
	}
	if len(missingInputs) > 0 {
		testSendDummyTransactionsWithRetry(t, missingNodes, domain, missingInputs, amount, snapVersionMint)
	}
	return outputs
}

func testSendDummyTransactions(nodes []*Node, domain common.Address, inputs []*common.Input, amount string, snapVersionMint int) []*common.Input {
	outputs := make([]*common.Input, len(inputs))

	var wg sync.WaitGroup
	for i, node := range nodes {
		wg.Add(1)
		go func(i int, node *Node) {
			raw, _ := json.Marshal(map[string]any{
				"version": 2,
				"asset":   "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
				"inputs": []map[string]any{{
					"hash":  inputs[i].Hash.String(),
					"index": inputs[i].Index,
				}},
				"outputs": []map[string]any{{
					"type":     0,
					"amount":   amount,
					"script":   "fffe01",
					"accounts": []string{domain.String()},
				}},
			})
			tx, _ := testSignTransaction(node.Host, domain, string(raw), snapVersionMint)
			ver := common.VersionedTransaction{SignedTransaction: *tx}
			id, _ := testSendTransaction(node.Host, hex.EncodeToString(ver.Marshal()))
			var res map[string]string
			json.Unmarshal([]byte(id), &res)
			hash, _ := crypto.HashFromString(res["hash"])
			outputs[i] = &common.Input{Index: 0, Hash: hash}
			wg.Done()
		}(i, node)
	}
	wg.Wait()

	return outputs
}

const configDataTmpl = `[node]
signer-key = "%s"
consensus-only = false
memory-cache-size = 128
kernel-operation-period = 3
cache-ttl = 3600
[network]
listener = "%s"
metric = true
peers = [%s]
`

func testPledgeNewNode(t *testing.T, nodes []*Node, domain common.Address, genesisData []byte, plist, input, root string, snapVersionMint int) (Node, *kernel.Node, *http.Server) {
	assert := assert.New(t)
	var signer, payee common.Address

	signer = testDetermineAccountByIndex(NODES, "SIGNER")
	payee = testDetermineAccountByIndex(NODES, "PAYEE")

	dir := fmt.Sprintf("%s/mixin-17099", root)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		panic(err)
	}

	configData := []byte(fmt.Sprintf(configDataTmpl, signer.PrivateSpendKey.String(), "127.0.0.1:17099", plist))
	err = os.WriteFile(dir+"/config.toml", configData, 0644)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(dir+"/genesis.json", genesisData, 0644)
	if err != nil {
		panic(err)
	}

	raw, _ := json.Marshal(map[string]any{
		"version": 2,
		"asset":   "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
		"inputs": []map[string]any{{
			"hash":  input,
			"index": NODES,
		}},
		"outputs": []map[string]any{{
			"type":   common.OutputTypeNodePledge,
			"amount": "10000",
		}},
		"extra": signer.PublicSpendKey.String() + payee.PublicSpendKey.String(),
	})
	tx, err := testSignTransaction(nodes[0].Host, domain, string(raw), snapVersionMint)
	assert.Nil(err)
	ver := common.VersionedTransaction{SignedTransaction: *tx}
	testSendTransactionsToNodesWithRetry(t, nodes, []*common.VersionedTransaction{&ver})

	custom, err := config.Initialize(dir + "/config.toml")
	assert.Nil(err)
	cache := newCache(custom)
	store, err := storage.NewBadgerStore(custom, dir)
	assert.Nil(err)
	assert.NotNil(store)
	pnode, err := kernel.SetupNode(custom, store, cache, fmt.Sprintf(":170%02d", 99), dir)
	assert.Nil(err)
	assert.NotNil(pnode)
	err = pnode.PingNeighborsFromConfig()
	assert.Nil(err)
	go pnode.Loop()

	server := NewServer(custom, store, pnode, 18099)
	go server.ListenAndServe()

	return Node{Signer: signer, Payee: payee, Host: "127.0.0.1:18099"}, pnode, server
}

func testBuildPledgeInput(t *testing.T, nodes []*Node, domain common.Address, utxos []*common.VersionedTransaction, snapVersionMint int) (string, error) {
	assert := assert.New(t)
	inputs := []map[string]any{}
	for _, tx := range utxos {
		inputs = append(inputs, map[string]any{
			"hash":  tx.PayloadHash().String(),
			"index": 0,
		})
	}
	outputs := []map[string]any{}
	for i := 0; i < NODES; i++ {
		outputs = append(outputs, map[string]any{
			"type":     0,
			"amount":   common.NewIntegerFromString("3.5").Div(NODES),
			"script":   "fffe01",
			"accounts": []string{domain.String()},
		})
	}
	outputs = append(outputs, map[string]any{
		"type":     0,
		"amount":   "10000",
		"script":   "fffe01",
		"accounts": []string{domain.String()},
	})
	raw, _ := json.Marshal(map[string]any{
		"version": 2,
		"asset":   "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
		"inputs":  inputs,
		"outputs": outputs,
	})
	tx, err := testSignTransaction(nodes[0].Host, domain, string(raw), snapVersionMint)
	assert.Nil(err)
	ver := common.VersionedTransaction{SignedTransaction: *tx}
	testSendTransactionsToNodesWithRetry(t, nodes, []*common.VersionedTransaction{&ver})
	return ver.PayloadHash().String(), err
}

func testSendTransactionsToNodesWithRetry(t *testing.T, nodes []*Node, vers []*common.VersionedTransaction) {
	assert := assert.New(t)

	var wg sync.WaitGroup
	for _, ver := range vers {
		wg.Add(1)
		go func(ver *common.VersionedTransaction) {
			node := nodes[int(time.Now().UnixNano())%len(nodes)].Host
			id, err := testSendTransaction(node, hex.EncodeToString(ver.Marshal()))
			assert.Nil(err)
			assert.Len(id, 75)
			defer wg.Done()
		}(ver)
	}
	wg.Wait()
	time.Sleep(3 * time.Second)

	var missingTxs []*common.VersionedTransaction
	for _, ver := range vers {
		node := nodes[int(time.Now().UnixNano())%len(nodes)].Host
		data, _ := callRPC(node, "gettransaction", []any{ver.PayloadHash().String()})
		var res map[string]string
		json.Unmarshal([]byte(data), &res)
		hash, _ := crypto.HashFromString(res["snapshot"])
		if hash.HasValue() {
			continue
		}
		t.Logf("TX MISSING %s\n", ver.PayloadHash())
		missingTxs = append(missingTxs, ver)
	}
	if len(missingTxs) == 0 {
		return
	}
	testSendTransactionsToNodesWithRetry(t, nodes, missingTxs)
}

func testSendTransactionToNodes(t *testing.T, nodes []*Node, raw string) string {
	assert := assert.New(t)

	rand.Seed(time.Now().UnixNano())
	for n := len(nodes); n > 0; n-- {
		ri := rand.Intn(n)
		nodes[n-1], nodes[ri] = nodes[ri], nodes[n-1]
	}
	wg, dup := &sync.WaitGroup{}, 3
	start := rand.Intn(len(nodes) - dup)

	var txHash string
	for i := start; i < len(nodes) && i != start+dup; i++ {
		wg.Add(1)
		go func(n string, raw string) {
			defer wg.Done()
			id, err := testSendTransaction(n, raw)
			assert.Nil(err)
			assert.Len(id, 75)
			txHash = id
		}(nodes[i].Host, raw)
	}
	wg.Wait()

	return txHash
}

func testSendTransaction(node, raw string) (string, error) {
	data, err := callRPC(node, "sendrawtransaction", []any{
		raw,
	})
	return string(data), err
}

func testGetNodeToRemove(networkId crypto.Hash, signers, payees []common.Address, seq int) (common.Address, common.Address) {
	nodes := make([][2]common.Address, len(signers))
	for i := range signers {
		nodes[i] = [2]common.Address{signers[i], payees[i]}
	}
	sort.Slice(nodes, func(i, j int) bool {
		a := nodes[i][0].Hash().ForNetwork(networkId)
		b := nodes[j][0].Hash().ForNetwork(networkId)
		return a.String() < b.String()
	})
	return nodes[seq][0], nodes[seq][1]
}

func testDetermineAccountByIndex(i int, role string) common.Address {
	seed := make([]byte, 64)
	copy(seed, []byte("TESTNODE#"+role+"#"))
	seed[63] = byte(i)
	account := common.NewAddressFromSeed(seed)
	account.PrivateViewKey = account.PublicSpendKey.DeterministicHashDerive()
	account.PublicViewKey = account.PrivateViewKey.Public()
	return account
}

func setupTestNet(root string) ([]common.Address, []common.Address, []byte, string) {
	var signers, payees []common.Address

	for i := 0; i < NODES; i++ {
		signers = append(signers, testDetermineAccountByIndex(i, "SIGNER"))
		payees = append(payees, testDetermineAccountByIndex(i, "PAYEE"))
	}

	inputs := make([]map[string]string, 0)
	for i := range signers {
		inputs = append(inputs, map[string]string{
			"signer":  signers[i].String(),
			"payee":   payees[i].String(),
			"balance": "10000",
		})
	}
	genesis := map[string]any{
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

	peers := make([]string, len(signers))
	for i := range signers {
		peers[i] = fmt.Sprintf("127.0.0.1:170%02d", i+1)
	}
	peersList := `"` + strings.Join(peers, `","`) + `"`

	for i, a := range signers {
		dir := fmt.Sprintf("%s/mixin-170%02d", root, i+1)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}

		configData := []byte(fmt.Sprintf(configDataTmpl, a.PrivateSpendKey.String(), peers[i], peersList))
		err = os.WriteFile(dir+"/config.toml", configData, 0644)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(dir+"/genesis.json", genesisData, 0644)
		if err != nil {
			panic(err)
		}
	}
	return signers, payees, genesisData, peersList
}

func testSignTransaction(node string, account common.Address, rawStr string, snapVersionMint int) (*common.SignedTransaction, error) {
	var raw signerInput
	err := json.Unmarshal([]byte(rawStr), &raw)
	if err != nil {
		panic(err)
	}
	raw.Node = node

	tx := common.NewTransactionV3(raw.Asset)
	if snapVersionMint < 1 && time.Now().UnixNano()%3 == 1 {
		tx = common.NewTransactionV2(raw.Asset)
	}
	if snapVersionMint < 1 && time.Now().UnixNano()%3 == 2 {
		tx = common.NewTransactionV4(raw.Asset)
	}
	for _, in := range raw.Inputs {
		if d := in.Deposit; d != nil {
			tx.AddDepositInput(&common.DepositData{
				Chain:           d.Chain,
				AssetKey:        d.AssetKey,
				TransactionHash: d.TransactionHash,
				OutputIndex:     d.OutputIndex,
				Amount:          d.Amount,
			})
		} else {
			tx.AddInput(in.Hash, in.Index)
		}
	}

	for _, out := range raw.Outputs {
		if out.Mask.HasValue() {
			panic("not here")
		}
		hash := crypto.NewHash([]byte(rawStr))
		seed := append(hash[:], hash[:]...)
		tx.AddOutputWithType(out.Type, out.Accounts, out.Script, out.Amount, seed)
	}

	extra, err := hex.DecodeString(raw.Extra)
	if err != nil {
		panic(err)
	}
	tx.Extra = extra

	signed := &common.SignedTransaction{Transaction: *tx}
	for i := range signed.Inputs {
		err := signed.SignInput(raw, i, []*common.Address{&account})
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
		assert.Equal(info.PoolSize, a.PoolSize)
	}
	return info
}

func testVerifySnapshots(assert *assert.Assertions, nodes []*Node) (map[string]bool, map[string]bool) {
	filters := make([]map[string]*common.Snapshot, 0)
	for _, n := range nodes {
		filters = append(filters, testListSnapshots(n.Host))
	}
	t, s := make(map[string]bool), make(map[string]bool)
	for i := 0; i < len(filters)-1; i++ {
		a, b := filters[i], filters[i+1]
		m, n := make(map[string]bool), make(map[string]bool)
		for k := range a {
			s[k] = true
			t[a[k].SoleTransaction().String()] = true
			m[a[k].SoleTransaction().String()] = true
		}
		for k := range b {
			s[k] = true
			t[b[k].SoleTransaction().String()] = true
			n[b[k].SoleTransaction().String()] = true
		}
		assertKeyEqual(assert, a, b)
		assert.Equal(len(a), len(b))
		assert.Equal(len(m), len(n))
		assert.Equal(len(filters[i]), len(filters[i+1]))
	}
	return t, s
}

func assertKeyEqual(assert *assert.Assertions, a, b map[string]*common.Snapshot) {
	var as, bs []string
	for k := range a {
		as = append(as, k)
	}
	for k := range b {
		bs = append(bs, k)
	}
	sort.Slice(as, func(i, j int) bool { return as[i] < as[j] })
	sort.Slice(bs, func(i, j int) bool { return bs[i] < bs[j] })
	assert.True(strings.Join(as, "") == strings.Join(bs, ""))
}

func testListSnapshots(node string) map[string]*common.Snapshot {
	data, err := callRPC(node, "listsnapshots", []any{
		0,
		100000,
		false,
		false,
	})

	var rss []*struct {
		Version           uint8                 `json:"version"`
		NodeId            crypto.Hash           `json:"node_id"`
		References        *common.RoundLink     `json:"references"`
		RoundNumber       uint64                `json:"round_number"`
		Timestamp         uint64                `json:"timestamp"`
		Signatures        []*crypto.Signature   `json:"signatures"`
		Signature         *crypto.CosiSignature `json:"signature"`
		Hash              crypto.Hash           `json:"hash"`
		Transactions      []crypto.Hash         `json:"transactions"`
		TransactionLegacy crypto.Hash           `json:"transaction"`
	}
	err = json.Unmarshal(data, &rss)
	if err != nil {
		panic(err)
	}
	filter := make(map[string]*common.Snapshot)
	snapshots := make([]*common.Snapshot, len(rss))
	for i, s := range rss {
		snapshots[i] = &common.Snapshot{
			Version:     s.Version,
			NodeId:      s.NodeId,
			RoundNumber: s.RoundNumber,
			References:  s.References,
			Timestamp:   s.Timestamp,
		}
		switch s.Version {
		case 0:
			snapshots[i].Signatures = s.Signatures
		case 1, 2:
			snapshots[i].Signature = s.Signature
		default:
			panic(s.Version)
		}
		snapshots[i].AddSoleTransaction(s.TransactionLegacy)
		filter[s.Hash.String()] = snapshots[i]
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
	data, err := callRPC(node, "listallnodes", []any{0, false})
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

type HeadRound struct {
	Node  crypto.Hash `json:"node"`
	Round uint64      `json:"round"`
	Hash  crypto.Hash `json:"hash"`
}

func testDumpGraphHead(node string, id crypto.Hash) *HeadRound {
	data, err := callRPC(node, "dumpgraphhead", []any{})
	if err != nil {
		panic(err)
	}
	var head []*HeadRound
	err = json.Unmarshal(data, &head)
	if err != nil {
		panic(err)
	}
	for _, r := range head {
		if r.Node == id {
			return r
		}
	}
	return nil
}

type Info struct {
	Timestamp time.Time
	PoolSize  common.Integer
}

func testGetGraphInfo(node string) Info {
	data, err := callRPC(node, "getinfo", []any{})
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

func testListMintDistributions(node string) []*common.Transaction {
	data, err := callRPC(node, "listmintdistributions", []any{
		0,
		10,
		false,
	})

	var mints []*struct {
		Group       string `json:"group"`
		Batch       int    `json:"batch"`
		Amount      string `json:"amount"`
		Transaction string `json:"transaction"`
	}
	err = json.Unmarshal(data, &mints)
	if err != nil {
		panic(err)
	}

	txs := make([]*common.Transaction, len(mints))
	for i, m := range mints {
		data, err := callRPC(node, "gettransaction", []any{m.Transaction})
		if err != nil {
			panic(err)
		}
		var tx struct {
			Inputs []*struct {
				Mint *common.MintData `json:"mint"`
			}
			Outputs []*struct {
				Type   uint8          `json:"type"`
				Amount common.Integer `json:"amount"`
				Keys   []*crypto.Key  `json:"keys"`
				Script common.Script  `json:"script"`
			}
		}
		err = json.Unmarshal(data, &tx)
		if err != nil {
			panic(err)
		}
		ctx := &common.Transaction{}
		for _, in := range tx.Inputs {
			ctx.Inputs = append(ctx.Inputs, &common.Input{Mint: in.Mint})
		}
		for _, out := range tx.Outputs {
			ctx.Outputs = append(ctx.Outputs, &common.Output{
				Type:   out.Type,
				Amount: out.Amount,
				Keys:   out.Keys,
				Script: out.Script,
			})
		}
		txs[i] = ctx
	}
	return txs
}

var httpClient *http.Client

func callRPC(node, method string, params []any) ([]byte, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	body, err := json.Marshal(map[string]any{
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
		Data  any `json:"data"`
		Error any `json:"error"`
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
		Hash    crypto.Hash `json:"hash"`
		Index   int         `json:"index"`
		Deposit *struct {
			Chain           crypto.Hash    `json:"chain"`
			AssetKey        string         `json:"asset"`
			TransactionHash string         `json:"transaction"`
			OutputIndex     uint64         `json:"index"`
			Amount          common.Integer `json:"amount"`
		} `json:"deposit,omitempty"`
		Keys []*crypto.Key `json:"keys"`
		Mask crypto.Key    `json:"mask"`
	} `json:"inputs"`
	Outputs []struct {
		Type     uint8             `json:"type"`
		Mask     crypto.Key        `json:"mask"`
		Keys     []*crypto.Key     `json:"keys"`
		Amount   common.Integer    `json:"amount"`
		Script   common.Script     `json:"script"`
		Accounts []*common.Address `json:"accounts"`
	}
	Asset crypto.Hash `json:"asset"`
	Extra string      `json:"extra"`
	Node  string      `json:"-"`
}

func (raw signerInput) ReadUTXOKeys(hash crypto.Hash, index int) (*common.UTXOKeys, error) {
	utxo := &common.UTXOKeys{}

	for _, in := range raw.Inputs {
		if in.Hash == hash && in.Index == index && len(in.Keys) > 0 {
			utxo.Keys = in.Keys
			utxo.Mask = in.Mask
			return utxo, nil
		}
	}

	data, err := callRPC(raw.Node, "getutxo", []any{hash.String(), index})
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

func newCache(conf *config.Custom) *ristretto.Cache {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7, // number of keys to track frequency of (10M).
		MaxCost:     int64(conf.Node.MemoryCacheSize) * 1024 * 1024,
		BufferItems: 64, // number of keys per Get buffer.
	})
	if err != nil {
		panic(err)
	}
	return cache
}
