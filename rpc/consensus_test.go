package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"slices"
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
	"github.com/stretchr/testify/require"
)

var (
	NODES  = 8
	INPUTS = 100
)

func TestConsensus(t *testing.T) {
	testConsensus(t)
}

func testConsensus(t *testing.T) {
	require := require.New(t)
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
	require.Nil(err)
	defer os.RemoveAll(root)

	accounts, payees, gdata, plist := setupTestNet(root)
	require.Len(accounts, NODES)

	epoch := time.Unix(1551312000, 0)
	nodes := make([]*Node, 0)
	instances := make([]*kernel.Node, 0)
	for i := range accounts {
		dir := fmt.Sprintf("%s/mixin-170%02d", root, i+1)
		custom, err := config.Initialize(dir + "/config.toml")
		require.Nil(err)
		cache := newCache(custom)
		store, err := storage.NewBadgerStore(custom, dir)
		require.Nil(err)
		require.NotNil(store)
		if i == 0 {
			kernel.TestMockDiff(epoch.Sub(time.Now()))
		}
		node, err := kernel.SetupNode(custom, store, cache, fmt.Sprintf(":170%02d", i+1), dir)
		require.Nil(err)
		require.NotNil(node)
		err = node.PingNeighborsFromConfig()
		require.Nil(err)
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
	tl, sl := testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))
	require.Equal(transactionsCount, len(sl))
	gt := testVerifyInfo(require, nodes)
	require.Truef(gt.Timestamp.Before(epoch.Add(1*time.Second)), "%s should before %s", gt.Timestamp, epoch.Add(1*time.Second))

	genesisAmount := (13439 + 3.5) / float64(INPUTS)
	domainAddress := accounts[0].String()
	deposits := make([]*common.VersionedTransaction, 0)
	for i := 0; i < INPUTS; i++ {
		raw := fmt.Sprintf(`{"version":5,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"deposit":{"chain":"8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27","asset":"0xa974c709cfb4566686553a20790685a47aceaa33","transaction":"0xc7c1132b58e1f64c263957d7857fe5ec5294fce95d30dcd64efef71da1%06d","index":0,"amount":"%f"}}],"outputs":[{"type":0,"amount":"%f","script":"fffe01","accounts":["%s"]}]}`, i, genesisAmount, genesisAmount, domainAddress)
		rand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[rand.Intn(len(nodes))].Host, accounts[0], raw)
		require.Nil(err)
		require.NotNil(tx)
		deposits = append(deposits, &common.VersionedTransaction{SignedTransaction: *tx})
	}

	testSendTransactionsToNodesWithRetry(t, nodes, deposits[:INPUTS/2])
	testSendTransactionsToNodesWithRetry(t, nodes, deposits[INPUTS/2:])
	transactionsCount = transactionsCount + INPUTS
	tl, _ = testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))

	gt = testVerifyInfo(require, nodes)
	require.Truef(gt.Timestamp.Before(epoch.Add(7*time.Second)), "%s should before %s", gt.Timestamp, epoch.Add(7*time.Second))
	hr := testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	require.NotNil(hr)
	require.GreaterOrEqual(hr.Round, uint64(0))
	t.Logf("DEPOSIT TEST DONE AT %s\n", time.Now())

	utxos := make([]*common.VersionedTransaction, 0)
	for _, d := range deposits {
		raw := fmt.Sprintf(`{"version":5,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"hash":"%s","index":0}],"outputs":[{"type":0,"amount":"%f","script":"fffe01","accounts":["%s"]}]}`, d.PayloadHash().String(), genesisAmount, domainAddress)
		rand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[rand.Intn(len(nodes))].Host, accounts[0], raw)
		require.Nil(err)
		require.NotNil(tx)
		if tx != nil {
			utxos = append(utxos, &common.VersionedTransaction{SignedTransaction: *tx})
		}
	}
	require.Equal(INPUTS, len(utxos))

	testSendTransactionsToNodesWithRetry(t, nodes, utxos[:INPUTS/2])
	testSendTransactionsToNodesWithRetry(t, nodes, utxos[INPUTS/2:])
	transactionsCount = transactionsCount + INPUTS
	tl, _ = testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))

	gt = testVerifyInfo(require, nodes)
	require.True(gt.Timestamp.Before(epoch.Add(31 * time.Second)))
	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	require.NotNil(hr)
	require.Greater(hr.Round, uint64(0))
	t.Logf("INPUT TEST DONE AT %s\n", time.Now())

	testCustodianUpdateNodes(t, nodes, accounts, payees, instances[0].NetworkId())
	transactionsCount = transactionsCount + 2
	t.Logf("CUSTODIAN TEST DONE AT %s\n", time.Now())

	if !enableElection {
		return
	}

	all := testListNodes(nodes[0].Host)
	require.Len(all, NODES)
	require.Equal("ACCEPTED", all[NODES-1].State)

	input, _ := testBuildPledgeInput(t, nodes, accounts[0], utxos)
	time.Sleep(3 * time.Second)
	transactionsCount = transactionsCount + 1
	tl, _ = testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))
	gt = testVerifyInfo(require, nodes)
	require.Less(gt.Timestamp, epoch.Add(31*time.Second))
	t.Logf("PLEDGE %s\n", input)

	dummyAmount := common.NewIntegerFromString("3.5").Div(NODES).String()
	dummyInputs := make([]*common.Input, NODES)
	for i := range dummyInputs {
		hash, _ := crypto.HashFromString(input)
		dummyInputs[i] = &common.Input{Hash: hash, Index: i}
	}

	legacy := time.Date(2023, time.Month(10), 31, 0, 0, 0, 0, time.UTC).Sub(epoch)
	kernel.TestMockDiff(legacy)
	for i := 0; i < 3; i++ {
		dummyInputs = testSendDummyTransactionsWithRetry(t, nodes, accounts[0], dummyInputs, dummyAmount)
		transactionsCount = transactionsCount + len(dummyInputs)
	}

	mints := testListMintDistributions(nodes[0].Host)
	require.Len(mints, 0)

	logger.SetLevel(logger.ERROR)
	logger.SetFilter("(?i)mint")

	kernel.TestMockDiff(time.Hour * (24 + config.KernelMintTimeBegin))
	tl, _ = testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))
	gt = testVerifyInfo(require, nodes)
	require.Less(gt.Timestamp, epoch.Add(legacy).Add(61*time.Second))

	pn, pi, sv := testPledgeNewNode(t, nodes, accounts[0], gdata, plist, input, root)
	t.Logf("PLEDGE %s\n", pn.Signer)
	transactionsCount = transactionsCount + 1
	defer pi.Teardown()
	defer sv.Close()

	for i := 0; i < 5; i++ {
		dummyInputs = testSendDummyTransactionsWithRetry(t, nodes, accounts[0], dummyInputs, dummyAmount)
		transactionsCount = transactionsCount + len(dummyInputs)
	}

	testCheckMintDistributions(require, nodes[0].Host)

	transactionsCount = transactionsCount + 1
	tl, _ = testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))
	gt = testVerifyInfo(require, nodes)
	require.Greater(gt.Timestamp, epoch.Add((config.KernelMintTimeBegin+24)*time.Hour))
	require.Equal("305850.45205696", gt.PoolSize.String())
	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	require.NotNil(hr)
	require.Greater(hr.Round, uint64(0))
	hr = testDumpGraphHead(nodes[0].Host, pi.IdForNetwork)
	require.Nil(hr)

	mints = testListMintDistributions(nodes[0].Host)
	require.Len(mints, 1)

	all = testListNodes(nodes[0].Host)
	require.Len(all, NODES+1)
	require.Equal(all[NODES].Signer.String(), pn.Signer.String())
	require.Equal(all[NODES].Payee.String(), pn.Payee.String())
	require.Equal("PLEDGING", all[NODES].State)
	t.Logf("PLEDGE TEST DONE AT %s\n", time.Now())

	kernel.TestMockDiff(29 * time.Hour)
	time.Sleep(3 * time.Second)
	all = testListNodes(nodes[0].Host)
	require.Len(all, NODES+1)
	require.Equal(all[NODES].Signer.String(), pn.Signer.String())
	require.Equal(all[NODES].Payee.String(), pn.Payee.String())
	require.Equal("PLEDGING", all[NODES].State)
	hr = testDumpGraphHead(nodes[0].Host, pi.IdForNetwork)
	require.Nil(hr)

	kernel.TestMockDiff(1 * time.Hour)
	time.Sleep(5 * time.Second)
	all = testListNodes(nodes[0].Host)
	require.Len(all, NODES+1)
	require.Equal(all[NODES].Signer.String(), pn.Signer.String())
	require.Equal(all[NODES].Payee.String(), pn.Payee.String())
	require.Equal("ACCEPTED", all[NODES].State)
	require.Equal(len(testListSnapshots(nodes[NODES-1].Host)), len(testListSnapshots(pn.Host)))
	require.Equal(len(testListSnapshots(nodes[0].Host)), len(testListSnapshots(pn.Host)))
	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	require.NotNil(hr)
	require.Greater(hr.Round, uint64(0))
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, instances[0].IdForNetwork)
	require.NotNil(hr)
	require.Greater(hr.Round, uint64(0))
	hr = testDumpGraphHead(nodes[0].Host, pi.IdForNetwork)
	require.NotNil(hr)
	require.Equal(uint64(0), hr.Round)
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, pi.IdForNetwork)
	require.NotNil(hr)
	require.Equal(uint64(0), hr.Round)

	transactionsCount = transactionsCount + 1
	tl, _ = testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))
	gt = testVerifyInfo(require, nodes)
	require.Greater(gt.Timestamp, epoch.Add((config.KernelMintTimeBegin+24)*time.Hour))
	require.Equal("305850.45205696", gt.PoolSize.String())
	t.Logf("ACCEPT TEST DONE AT %s\n", time.Now())

	kernel.TestMockDiff(24 * time.Hour)
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		require.Len(all, NODES+1)
		require.Equal(all[NODES].Signer.String(), pn.Signer.String())
		require.Equal(all[NODES].Payee.String(), pn.Payee.String())
		require.Equal("ACCEPTED", all[NODES].State)
	}

	nodes = append(nodes, &Node{Host: "127.0.0.1:18099"})
	signer, payee := testGetNodeToRemove(instances[0].NetworkId(), accounts, payees, 0)
	require.Equal("XINW6HTiMVmKHjfnk3DYbcWcTaTkKi4dr3wZgicyhKvKnyYEqD8PD5ZRfL13ZsouiMURM6atDh3Bdr3dqSVkYWEm7Kzp9Axt", signer.String())
	require.Equal("XINCtoRSJYrNNQUv3xTsptxDKRqwHMwtNkvsQwFS58oFXYvgu9QhoetNwbmxUQ4JJGcjR1gnttMau1nCmGpkSimHR1dxrP8u", payee.String())
	nodes = testRemoveNode(nodes, signer)
	for i := 0; i < 3; i++ {
		dummyInputs = testSendDummyTransactionsWithRetry(t, nodes, accounts[0], dummyInputs, dummyAmount)
		transactionsCount = transactionsCount + len(dummyInputs)
	}
	transactionsCount = transactionsCount + 1

	tl, _ = testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		require.Len(all, NODES+1)
		require.Equal(all[NODES].Signer.String(), signer.String())
		require.Equal(all[NODES].Payee.String(), payee.String())
		require.Equal("REMOVED", all[NODES].State)
	}

	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	require.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, instances[0].IdForNetwork)
	require.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[0].Host, pi.IdForNetwork)
	require.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, pi.IdForNetwork)
	require.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[0].Host, signer.Hash().ForNetwork(instances[0].NetworkId()))
	require.Greater(hr.Round, uint64(1))
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, signer.Hash().ForNetwork(instances[0].NetworkId()))
	require.Greater(hr.Round, uint64(1))

	removalInputs := []*common.Input{{Hash: all[NODES].Transaction, Index: 0}}
	removalInputs = testSendDummyTransactionsWithRetry(t, nodes[:1], payee, removalInputs, "13439")
	transactionsCount = transactionsCount + 1
	tl, _ = testVerifySnapshots(require, nodes)
	require.Equal(transactionsCount, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		require.Len(all, NODES+1)
		require.Equal(all[NODES].Signer.String(), signer.String())
		require.Equal(all[NODES].Payee.String(), payee.String())
		require.Equal("REMOVED", all[NODES].State)
	}
	t.Logf("REMOVE TEST DONE AT %s\n", time.Now())

	for _, node := range instances {
		t.Log(node.IdForNetwork, node.Peer.Metric())
	}

}

func testCustodianUpdateNodes(t *testing.T, nodes []*Node, signers, payees []common.Address, networkId crypto.Hash) {
	require := require.New(t)
	tx := common.NewTransactionV5(common.XINAssetId)
	require.NotNil(tx)

	domain := signers[0]

	seed := make([]byte, 64)
	rand.Read(seed)
	count := len(signers)
	custodian := common.NewAddressFromSeed(seed)

	custodianNodes := make([]*common.CustodianNode, count)
	for i := 0; i < count; i++ {
		signer := signers[i]
		payee := payees[i]
		seed := make([]byte, 64)
		rand.Read(seed)
		custodian := common.NewAddressFromSeed(seed)
		extra := common.EncodeCustodianNode(&custodian, &payee, &signer.PrivateSpendKey, &payee.PrivateSpendKey, &custodian.PrivateSpendKey, networkId)
		custodianNodes[i] = &common.CustodianNode{Custodian: custodian, Payee: payee, Extra: extra}
	}

	amount := common.NewInteger(100).Mul(count)
	tx.AddScriptOutput([]*common.Address{&custodian}, common.NewThresholdScript(common.Operator64), amount, make([]byte, 64))
	tx.Outputs[0].Type = common.OutputTypeCustodianUpdateNodes

	sortedExtra := append(custodian.PublicSpendKey[:], custodian.PublicViewKey[:]...)
	sort.Slice(custodianNodes, func(i, j int) bool {
		return bytes.Compare(custodianNodes[i].Custodian.PublicSpendKey[:], custodianNodes[j].Custodian.PublicSpendKey[:]) < 0
	})
	for _, n := range custodianNodes {
		sortedExtra = append(sortedExtra, n.Extra...)
	}
	sh := crypto.Blake3Hash(sortedExtra)
	sig := domain.PrivateSpendKey.Sign(sh)
	tx.Extra = append(sortedExtra, sig[:]...)

	raw := fmt.Sprintf(`{"version":5,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"deposit":{"chain":"8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27","asset":"0xa974c709cfb4566686553a20790685a47aceaa33","transaction":"0xc7c1132b58e1f64c263957d7857fe5ec5294fce95d30dcd64efef71da1%06d","index":0,"amount":"%s"}}],"outputs":[{"type":0,"amount":"%s","script":"fffe01","accounts":["%s"]}]}`, 13439, amount.String(), amount.String(), domain.String())
	rand.Seed(time.Now().UnixNano())
	deposit, err := testSignTransaction(nodes[0].Host, domain, raw)
	require.Nil(err)
	require.NotNil(deposit)
	deposits := []*common.VersionedTransaction{{SignedTransaction: *deposit}}
	testSendTransactionsToNodesWithRetry(t, nodes, deposits)

	inputs := []map[string]any{{
		"hash":  deposit.AsVersioned().PayloadHash(),
		"index": 0,
	}}
	out := tx.Outputs[0]
	outputs := []map[string]any{{
		"type":     out.Type,
		"amount":   out.Amount,
		"script":   out.Script.String(),
		"accounts": []string{domain.String()},
	}}
	rb, _ := json.Marshal(map[string]any{
		"version": tx.Version,
		"asset":   tx.Asset,
		"inputs":  inputs,
		"outputs": outputs,
		"extra":   hex.EncodeToString(tx.Extra),
	})
	signed, err := testSignTransaction(nodes[0].Host, domain, string(rb))
	require.Nil(err)
	require.NotNil(signed)

	updates := []*common.VersionedTransaction{{SignedTransaction: *signed}}
	testSendTransactionsToNodesWithRetry(t, nodes, updates)

	raw = hex.EncodeToString(signed.AsVersioned().Marshal())
	id, err := testSendTransaction(nodes[0].Host, raw)
	require.Nil(err)
	require.Len(id, 75)
	var res map[string]string
	json.Unmarshal([]byte(id), &res)
	hash, _ := crypto.HashFromString(res["hash"])
	require.True(hash.HasValue())

	data, err := callRPC(nodes[0].Host, "listcustodianupdates", []any{})
	require.Nil(err)
	var curs []*struct {
		Custodian   string `json:"custodian"`
		Timestamp   uint64 `json:"timestamp"`
		Transaction string `json:"transaction"`
	}
	err = json.Unmarshal(data, &curs)
	require.Nil(err)
	require.Len(curs, 2)
	require.Equal(hash.String(), curs[1].Transaction)
	require.Equal(custodian.String(), curs[1].Custodian)
}

func testCheckMintDistributions(require *require.Assertions, node string) {
	mints := testListMintDistributions(node)
	require.Len(mints, 1)
	tx := mints[0]
	require.Len(tx.Inputs, 1)
	mint := tx.Inputs[0].Mint
	daily := common.NewIntegerFromString("89.87671232")
	require.Equal("UNIVERSAL", mint.Group)
	require.Equal(uint64(1707), mint.Batch)
	require.Equal(daily, mint.Amount)
	require.Len(tx.Outputs, NODES+2)
	total := common.Zero
	for i, o := range tx.Outputs {
		if i < NODES {
			total = total.Add(o.Amount)
			require.Equal("fffe01", o.Script.String())
			require.Equal(uint8(common.OutputTypeScript), o.Type)
			require.Len(o.Keys, 1)
		} else if i == NODES {
			require.Equal("fffe01", o.Script.String())
			require.Equal(daily.Div(10).Mul(4), o.Amount)
			require.Equal(uint8(common.OutputTypeScript), o.Type)
			require.Len(o.Keys, 1)
		} else if i == NODES+1 {
			custodian := daily.Div(10).Mul(4)
			total = total.Add(custodian)
			light := daily.Sub(total)
			require.Equal("fffe40", o.Script.String())
			require.Equal(light, o.Amount)
			require.Equal(uint8(common.OutputTypeScript), o.Type)
			require.Len(o.Keys, 1)
		}
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

func testSendDummyTransactionsWithRetry(t *testing.T, nodes []*Node, domain common.Address, inputs []*common.Input, amount string) []*common.Input {
	outputs := testSendDummyTransactions(nodes, domain, inputs, amount)
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
		testSendDummyTransactionsWithRetry(t, missingNodes, domain, missingInputs, amount)
	}
	return outputs
}

func testSendDummyTransactions(nodes []*Node, domain common.Address, inputs []*common.Input, amount string) []*common.Input {
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
			tx, _ := testSignTransaction(node.Host, domain, string(raw))
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

func testPledgeNewNode(t *testing.T, nodes []*Node, domain common.Address, genesisData []byte, plist, input, root string) (Node, *kernel.Node, *http.Server) {
	require := require.New(t)
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
			"amount": "13439",
		}},
		"extra": signer.PublicSpendKey.String() + payee.PublicSpendKey.String(),
	})
	tx, err := testSignTransaction(nodes[0].Host, domain, string(raw))
	require.Nil(err)
	ver := common.VersionedTransaction{SignedTransaction: *tx}
	testSendTransactionsToNodesWithRetry(t, nodes, []*common.VersionedTransaction{&ver})

	custom, err := config.Initialize(dir + "/config.toml")
	require.Nil(err)
	cache := newCache(custom)
	store, err := storage.NewBadgerStore(custom, dir)
	require.Nil(err)
	require.NotNil(store)
	pnode, err := kernel.SetupNode(custom, store, cache, fmt.Sprintf(":170%02d", 99), dir)
	require.Nil(err)
	require.NotNil(pnode)
	err = pnode.PingNeighborsFromConfig()
	require.Nil(err)
	go pnode.Loop()

	server := NewServer(custom, store, pnode, 18099)
	go server.ListenAndServe()

	return Node{Signer: signer, Payee: payee, Host: "127.0.0.1:18099"}, pnode, server
}

func testBuildPledgeInput(t *testing.T, nodes []*Node, domain common.Address, utxos []*common.VersionedTransaction) (string, error) {
	require := require.New(t)
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
		"amount":   "13439",
		"script":   "fffe01",
		"accounts": []string{domain.String()},
	})
	raw, _ := json.Marshal(map[string]any{
		"version": 2,
		"asset":   "a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc",
		"inputs":  inputs,
		"outputs": outputs,
	})
	tx, err := testSignTransaction(nodes[0].Host, domain, string(raw))
	require.Nil(err)
	ver := common.VersionedTransaction{SignedTransaction: *tx}
	testSendTransactionsToNodesWithRetry(t, nodes, []*common.VersionedTransaction{&ver})
	return ver.PayloadHash().String(), err
}

func testSendTransactionsToNodesWithRetry(t *testing.T, nodes []*Node, vers []*common.VersionedTransaction) {
	require := require.New(t)

	var wg sync.WaitGroup
	for _, ver := range vers {
		wg.Add(1)
		go func(ver *common.VersionedTransaction) {
			node := nodes[int(time.Now().UnixNano())%len(nodes)].Host
			id, err := testSendTransaction(node, hex.EncodeToString(ver.Marshal()))
			require.Nil(err)
			require.Len(id, 75)
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
	var signers, payees, custodians []common.Address

	for i := 0; i < NODES; i++ {
		signers = append(signers, testDetermineAccountByIndex(i, "SIGNER"))
		payees = append(payees, testDetermineAccountByIndex(i, "PAYEE"))
		custodians = append(custodians, testDetermineAccountByIndex(i, "CUSTODIAN"))
	}

	inputs := make([]map[string]string, 0)
	for i := range signers {
		inputs = append(inputs, map[string]string{
			"signer":    signers[i].String(),
			"payee":     payees[i].String(),
			"custodian": custodians[i].String(),
			"balance":   "13439",
		})
	}

	domain := signers[0]
	genesis := map[string]any{
		"epoch":     1551312000,
		"nodes":     inputs,
		"custodian": domain.String(),
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

func testSignTransaction(node string, account common.Address, rawStr string) (*common.SignedTransaction, error) {
	var raw signerInput
	err := json.Unmarshal([]byte(rawStr), &raw)
	if err != nil {
		panic(err)
	}
	raw.Node = node

	tx := common.NewTransactionV5(raw.Asset)
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
		hash := crypto.Blake3Hash([]byte(rawStr))
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

func testVerifyInfo(require *require.Assertions, nodes []*Node) Info {
	info := testGetGraphInfo(nodes[0].Host)
	for _, n := range nodes {
		a := testGetGraphInfo(n.Host)
		require.Equal(info.PoolSize, a.PoolSize)
	}
	return info
}

func testVerifySnapshots(require *require.Assertions, nodes []*Node) (map[string]bool, map[string]bool) {
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
		requireKeyEqual(require, a, b)
		require.Equal(len(a), len(b))
		require.Equal(len(m), len(n))
		require.Equal(len(filters[i]), len(filters[i+1]))
	}
	return t, s
}

func requireKeyEqual(require *require.Assertions, a, b map[string]*common.Snapshot) {
	var as, bs []string
	for k := range a {
		as = append(as, k)
	}
	for k := range b {
		bs = append(bs, k)
	}
	slices.Sort(as)
	slices.Sort(bs)
	require.Equal(len(as), len(bs))
	require.True(strings.Join(as, "") == strings.Join(bs, ""))
}

func testListSnapshots(node string) map[string]*common.Snapshot {
	data, err := callRPC(node, "listsnapshots", []any{
		0,
		100000,
		false,
		false,
	})

	var rss []*struct {
		Version      uint8                 `json:"version"`
		NodeId       crypto.Hash           `json:"node_id"`
		RoundNumber  uint64                `json:"round_number"`
		References   *common.RoundLink     `json:"references"`
		Timestamp    uint64                `json:"timestamp"`
		Transactions []crypto.Hash         `json:"transactions"`
		Signature    *crypto.CosiSignature `json:"signature"`
		Hash         crypto.Hash           `json:"hash"`
	}
	err = json.Unmarshal(data, &rss)
	if err != nil {
		panic(err)
	}
	filter := make(map[string]*common.Snapshot)
	snapshots := make([]*common.Snapshot, len(rss))
	for i, s := range rss {
		snapshots[i] = &common.Snapshot{
			Version:      s.Version,
			NodeId:       s.NodeId,
			RoundNumber:  s.RoundNumber,
			References:   s.References,
			Timestamp:    s.Timestamp,
			Transactions: s.Transactions,
			Signature:    s.Signature,
		}
		switch s.Version {
		case 2:
			snapshots[i].Signature = s.Signature
		default:
			panic(s.Version)
		}
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
	data, err := callRPC(node, "listallnodes", []any{time.Now().UnixNano() * 2, false})
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
