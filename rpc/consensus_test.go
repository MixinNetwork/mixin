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
	"github.com/VictoriaMetrics/fastcache"
	"github.com/stretchr/testify/assert"
)

const (
	NODES  = 8
	INPUTS = 100
)

func TestAllTransactionsToSingleGenesisNode(t *testing.T) {
	assert := assert.New(t)

	kernel.TestMockReset()

	root, err := os.MkdirTemp("", "mixin-attsg-test")
	assert.Nil(err)
	defer os.RemoveAll(root)

	accounts, _, _, _ := setupTestNet(root)
	assert.Len(accounts, NODES)

	epoch := time.Unix(1551312000, 0)
	nodes := make([]*Node, 0)
	instances := make([]*kernel.Node, 0)
	for i := range accounts {
		dir := fmt.Sprintf("%s/mixin-170%02d", root, i+1)
		custom, err := config.Initialize(dir + "/config.toml")
		assert.Nil(err)
		cache := fastcache.New(custom.Node.MemoryCacheSize * 1024 * 1024)
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

	rand.Seed(time.Now().UnixNano())
	target := nodes[rand.Intn(len(nodes))]

	tl, sl := testVerifySnapshots(assert, nodes)
	assert.Equal(NODES+1, len(tl))
	assert.Equal(NODES+1, len(sl))
	gt := testVerifyInfo(assert, nodes)
	assert.Truef(gt.Timestamp.Before(epoch.Add(1*time.Second)), "%s should before %s", gt.Timestamp, epoch.Add(1*time.Second))

	genesisAmount := float64(10003.5) / INPUTS
	domainAddress := accounts[0].String()
	deposits := make([]*common.VersionedTransaction, 0)
	for i := 0; i < INPUTS; i++ {
		raw := fmt.Sprintf(`{"version":2,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"deposit":{"chain":"8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27","asset":"0xa974c709cfb4566686553a20790685a47aceaa33","transaction":"0xc7c1132b58e1f64c263957d7857fe5ec5294fce95d30dcd64efef71da1%06d","index":0,"amount":"%f"}}],"outputs":[{"type":0,"amount":"%f","script":"fffe01","accounts":["%s"]}]}`, i, genesisAmount, genesisAmount, domainAddress)
		tx, err := testSignTransaction(target.Host, accounts[0], raw)
		assert.Nil(err)
		assert.NotNil(tx)
		deposits = append(deposits, &common.VersionedTransaction{SignedTransaction: *tx})
	}

	for _, d := range deposits {
		n, raw := target.Host, hex.EncodeToString(d.Marshal())
		id, err := testSendTransaction(n, raw)
		assert.Nil(err)
		assert.Len(id, 75)
	}

	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS+NODES+1, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.Truef(gt.Timestamp.Before(epoch.Add(1*time.Second)), "%s should before %s", gt.Timestamp, epoch.Add(1*time.Second))

	utxos := make([]*common.VersionedTransaction, 0)
	for _, d := range deposits {
		raw := fmt.Sprintf(`{"version":2,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"hash":"%s","index":0}],"outputs":[{"type":0,"amount":"%f","script":"fffe01","accounts":["%s"]}]}`, d.PayloadHash().String(), genesisAmount, domainAddress)
		rand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[0].Host, accounts[0], raw)
		assert.Nil(err)
		assert.NotNil(tx)
		if tx != nil {
			utxos = append(utxos, &common.VersionedTransaction{SignedTransaction: *tx})
		}
	}
	assert.Equal(INPUTS, len(utxos))

	for _, tx := range utxos {
		n, raw := nodes[0].Host, hex.EncodeToString(tx.Marshal())
		id, err := testSendTransaction(n, raw)
		assert.Nil(err)
		assert.Len(id, 75)
	}

	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(31 * time.Second)))
}

func TestConsensusSingle(t *testing.T) {
	testConsensus(t, 1)
}

func TestConsensusDouble(t *testing.T) {
	testConsensus(t, 2)
}

func TestConsensusMany(t *testing.T) {
	testConsensus(t, -1)
}

func testConsensus(t *testing.T, dup int) {
	assert := assert.New(t)

	kernel.TestMockReset()

	level, _ := strconv.ParseInt(os.Getenv("LOG"), 10, 64)
	logger.SetLevel(int(level))
	logger.SetLimiter(10)

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
		cache := fastcache.New(custom.Node.MemoryCacheSize * 1024 * 1024)
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
		t.Logf("NODES#%d %s\n", i, node.IdForNetwork)

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

	tl, sl := testVerifySnapshots(assert, nodes)
	assert.Equal(NODES+1, len(tl))
	assert.Equal(NODES+1, len(sl))
	gt := testVerifyInfo(assert, nodes)
	assert.Truef(gt.Timestamp.Before(epoch.Add(1*time.Second)), "%s should before %s", gt.Timestamp, epoch.Add(1*time.Second))

	genesisAmount := float64(10003.5) / INPUTS
	domainAddress := accounts[0].String()
	deposits := make([]*common.VersionedTransaction, 0)
	for i := 0; i < INPUTS; i++ {
		raw := fmt.Sprintf(`{"version":2,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"deposit":{"chain":"8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27","asset":"0xa974c709cfb4566686553a20790685a47aceaa33","transaction":"0xc7c1132b58e1f64c263957d7857fe5ec5294fce95d30dcd64efef71da1%06d","index":0,"amount":"%f"}}],"outputs":[{"type":0,"amount":"%f","script":"fffe01","accounts":["%s"]}]}`, i, genesisAmount, genesisAmount, domainAddress)
		rand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[rand.Intn(len(nodes))].Host, accounts[0], raw)
		assert.Nil(err)
		assert.NotNil(tx)
		deposits = append(deposits, &common.VersionedTransaction{SignedTransaction: *tx})
	}

	for _, d := range deposits {
		rand.Seed(time.Now().UnixNano())
		for n := len(nodes); n > 0; n-- {
			randIndex := rand.Intn(n)
			nodes[n-1], nodes[randIndex] = nodes[randIndex], nodes[n-1]
		}
		wg := &sync.WaitGroup{}
		start := rand.Intn(len(nodes) - 1)
		for i := start; i < len(nodes) && i != start+dup; i++ {
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

	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS+NODES+1, len(tl))
	for i, d := range deposits {
		if !tl[d.PayloadHash().String()] {
			t.Logf("DEPOSIT MISSING %d %s\n", i, d.PayloadHash())
			id, err := testSendTransaction(nodes[0].Host, hex.EncodeToString(d.Marshal()))
			assert.Nil(err)
			assert.Contains(id, d.PayloadHash().String())
		}
	}
	gt = testVerifyInfo(assert, nodes)
	assert.Truef(gt.Timestamp.Before(epoch.Add(1*time.Second)), "%s should before %s", gt.Timestamp, epoch.Add(1*time.Second))
	hr := testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	assert.NotNil(hr)
	assert.GreaterOrEqual(hr.Round, uint64(0))
	t.Log("DEPOSIT TEST DONE", time.Now())

	utxos := make([]*common.VersionedTransaction, 0)
	for _, d := range deposits {
		raw := fmt.Sprintf(`{"version":2,"asset":"a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc","inputs":[{"hash":"%s","index":0}],"outputs":[{"type":0,"amount":"%f","script":"fffe01","accounts":["%s"]}]}`, d.PayloadHash().String(), genesisAmount, domainAddress)
		rand.Seed(time.Now().UnixNano())
		tx, err := testSignTransaction(nodes[rand.Intn(len(nodes))].Host, accounts[0], raw)
		assert.Nil(err)
		assert.NotNil(tx)
		if tx != nil {
			utxos = append(utxos, &common.VersionedTransaction{SignedTransaction: *tx})
		}
	}
	assert.Equal(INPUTS, len(utxos))

	for _, tx := range utxos {
		rand.Seed(time.Now().UnixNano())
		for n := len(nodes); n > 0; n-- {
			randIndex := rand.Intn(n)
			nodes[n-1], nodes[randIndex] = nodes[randIndex], nodes[n-1]
		}
		wg := &sync.WaitGroup{}
		start := rand.Intn(len(nodes) - 1)
		for i := start; i < len(nodes) && i != start+dup; i++ {
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

	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1, len(tl))
	for i, tx := range utxos {
		if !tl[tx.PayloadHash().String()] {
			t.Logf("UTXO MISSING %d %s\n", i, tx.PayloadHash())
			id, err := testSendTransaction(nodes[0].Host, hex.EncodeToString(tx.Marshal()))
			assert.Nil(err)
			assert.Contains(id, tx.PayloadHash().String())
		}
	}
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(31 * time.Second)))
	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	assert.NotNil(hr)
	assert.Greater(hr.Round, uint64(0))
	t.Log("INPUT TEST DONE", time.Now())

	kernel.TestMockDiff((config.KernelMintTimeBegin + 24) * time.Hour)
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(31 * time.Second)))

	all := testListNodes(nodes[0].Host)
	assert.Len(all, NODES)
	assert.Equal("ACCEPTED", all[NODES-1].State)

	input, err := testBuildPledgeInput(assert, nodes[0].Host, accounts[0], utxos)
	assert.Nil(err)
	time.Sleep(5 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.Before(epoch.Add(61 * time.Second)))
	t.Logf("PLEDGE %s\n", input)

	pn, pi, sv := testPledgeNewNode(assert, nodes[0].Host, accounts[0], gdata, plist, input, root)
	defer pi.Teardown()
	defer sv.Close()
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.After(epoch.Add((config.KernelMintTimeBegin + 24) * time.Hour)))
	assert.Equal("499876.71232883", gt.PoolSize.String())
	hr = testDumpGraphHead(nodes[0].Host, instances[0].IdForNetwork)
	assert.NotNil(hr)
	assert.Greater(hr.Round, uint64(0))
	hr = testDumpGraphHead(nodes[0].Host, pi.IdForNetwork)
	assert.Nil(hr)

	all = testListNodes(nodes[0].Host)
	assert.Len(all, NODES+1)
	assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
	assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
	assert.Equal("PLEDGING", all[NODES].State)
	t.Log("PLEDGE TEST DONE", time.Now())

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

	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1, len(tl))
	gt = testVerifyInfo(assert, nodes)
	assert.True(gt.Timestamp.After(epoch.Add((config.KernelMintTimeBegin + 24) * time.Hour)))
	assert.Equal("499876.71232883", gt.PoolSize.String())

	kernel.TestMockDiff(24 * time.Hour)
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1, len(tl))

	input = testSendDummyTransaction(assert, nodes[0].Host, accounts[0], input, "3.5")
	t.Logf("DUMMY 1 %s\n", input)
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1+1, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), pn.Signer.String())
		assert.Equal(all[NODES].Payee.String(), pn.Payee.String())
		assert.Equal("ACCEPTED", all[NODES].State)
	}
	t.Log("ACCEPT TEST DONE", time.Now())
	nodes = append(nodes, &Node{Host: "127.0.0.1:18099"})

	signer, payee := testGetNodeToRemove(instances[0].NetworkId(), accounts, payees, 0)
	assert.Equal("XINGmuYCB65rzMgUf1W35pbhj4C7fY9JrzWCL5vGRdL84SPcWVPhtBJ7DAarc1QPt564JwbEdNCH8359kdPRH1ieSM9f96RZ", signer.String())
	assert.Equal("XINMeKsKkSJJCgLWKvakEHaXBNPGfF7RmBu9jx5VZLE6UTuEaW4wSEqVybkH4xhQcqkT5jdiguiN3B3NKt8QBZTUbqZXJ1Fq", payee.String())
	input = testSendDummyTransaction(assert, nodes[0].Host, accounts[0], input, "3.5")
	t.Logf("DUMMY 2 %s\n", input)
	assert.Len(input, 64)
	nodes = testRemoveNode(nodes, signer)
	time.Sleep(5 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1+1+2, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), signer.String())
		assert.Equal(all[NODES].Payee.String(), payee.String())
		assert.Equal("REMOVED", all[NODES].State)
	}
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
	hr = testDumpGraphHead(nodes[0].Host, signer.Hash().ForNetwork(instances[0].NetworkId()))
	assert.NotNil(hr)
	assert.Greater(hr.Round, uint64(0))
	hr = testDumpGraphHead(nodes[len(nodes)-1].Host, signer.Hash().ForNetwork(instances[0].NetworkId()))
	assert.NotNil(hr)
	assert.Greater(hr.Round, uint64(0))

	kernel.TestMockDiff(24 * time.Hour)
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)

	removal := testSendDummyTransaction(assert, nodes[0].Host, payee, all[NODES].Transaction.String(), "10000")
	t.Logf("DUMMY 3 %s\n", removal)
	assert.Len(removal, 64)
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1+1+2+1, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), signer.String())
		assert.Equal(all[NODES].Payee.String(), payee.String())
		assert.Equal("REMOVED", all[NODES].State)
	}

	signer, payee = testGetNodeToRemove(instances[0].NetworkId(), accounts, payees, 1)
	assert.Equal("XINEBDHLoH3eDiicxWdbF7B79h1kZMT2rmuoQcvSPDYztBFvkgv8dESeCKW1Ejh8hQ7QUK2Lxd8aHa5heXmWYk2gc78wWVs", signer.String())
	assert.Equal("XINRqCuT9qD7qd9Z4bJFhAHq4eRa1GWdeD6cM6AegbS9chCFRFpxfxvFey6LouzRDYJgKXd45YXM1douCFvwELZmQ9gSCjPP", payee.String())
	input = testSendDummyTransaction(assert, nodes[0].Host, accounts[0], input, "3.5")
	t.Logf("DUMMY 4 %s\n", input)
	assert.Len(input, 64)
	nodes = testRemoveNode(nodes, signer)
	assert.Len(nodes, NODES+1-2)
	time.Sleep(5 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1+1+2+1+2, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), signer.String())
		assert.Equal(all[NODES].Payee.String(), payee.String())
		assert.Equal("REMOVED", all[NODES].State)
	}

	kernel.TestMockDiff(24 * time.Hour)
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)

	removal = testSendDummyTransaction(assert, nodes[0].Host, payee, all[NODES].Transaction.String(), "10000")
	t.Logf("DUMMY 5 %s\n", removal)
	assert.Len(removal, 64)
	time.Sleep(3 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1+1+2+1+2+1, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), signer.String())
		assert.Equal(all[NODES].Payee.String(), payee.String())
		assert.Equal("REMOVED", all[NODES].State)
	}

	input = testSendDummyTransaction(assert, nodes[0].Host, accounts[0], input, "3.5")
	t.Logf("DUMMY 6 %s\n", input)
	assert.Len(input, 64)
	nodes = testRemoveNode(nodes, signer)
	time.Sleep(5 * time.Second)
	tl, _ = testVerifySnapshots(assert, nodes)
	assert.Equal(INPUTS*2+NODES+1+1+2+1+1+2+1+2+1+1, len(tl))
	for i := range nodes {
		all = testListNodes(nodes[i].Host)
		assert.Len(all, NODES+1)
		assert.Equal(all[NODES].Signer.String(), signer.String())
		assert.Equal(all[NODES].Payee.String(), payee.String())
		assert.Equal("REMOVED", all[NODES].State)
	}
	t.Log("REMOVE TEST DONE", time.Now())
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

func testSendDummyTransaction(assert *assert.Assertions, node string, domain common.Address, th, amount string) string {
	raw, _ := json.Marshal(map[string]interface{}{
		"version": 2,
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

const configDataTmpl = `[node]
signer-key = "%s"
consensus-only = false
memory-cache-size = 128
kernel-operation-period = 1
cache-ttl = 3600
[network]
listener = "%s"
peers = [%s]
`

func testPledgeNewNode(assert *assert.Assertions, node string, domain common.Address, genesisData []byte, plist, input, root string) (Node, *kernel.Node, *http.Server) {
	var signer, payee common.Address

	signer = testDeterminAccountByIndex(NODES, "SIGNER")
	payee = testDeterminAccountByIndex(NODES, "PAYEE")

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

	raw, _ := json.Marshal(map[string]interface{}{
		"version": 2,
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

	custom, err := config.Initialize(dir + "/config.toml")
	assert.Nil(err)
	cache := fastcache.New(custom.Node.MemoryCacheSize * 1024 * 1024)
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

func testBuildPledgeInput(assert *assert.Assertions, node string, domain common.Address, utxos []*common.VersionedTransaction) (string, error) {
	inputs := []map[string]interface{}{}
	for _, tx := range utxos {
		inputs = append(inputs, map[string]interface{}{
			"hash":  tx.PayloadHash().String(),
			"index": 0,
		})
	}
	raw, _ := json.Marshal(map[string]interface{}{
		"version": 2,
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

func testDeterminAccountByIndex(i int, role string) common.Address {
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
		signers = append(signers, testDeterminAccountByIndex(i, "SIGNER"))
		payees = append(payees, testDeterminAccountByIndex(i, "PAYEE"))
	}

	inputs := make([]map[string]string, 0)
	for i := range signers {
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

	tx := common.NewTransaction(raw.Asset)
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
		assert.Equal(info.Timestamp, a.Timestamp)
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
			t[a[k].Transaction.String()] = true
			m[a[k].Transaction.String()] = true
		}
		for k := range b {
			s[k] = true
			t[b[k].Transaction.String()] = true
			n[b[k].Transaction.String()] = true
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
	data, err := callRPC(node, "listallnodes", []interface{}{0, false})
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
	data, err := callRPC(node, "dumpgraphhead", []interface{}{})
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
