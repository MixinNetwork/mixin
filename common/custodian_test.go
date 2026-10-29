package common

import (
	"bytes"
	"encoding/hex"
	"sort"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

const mainnetId = "6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997"

func TestCustodianUpdateNodes(t *testing.T) {
	require := require.New(t)

	tx := NewTransactionV5(XINAssetId)
	require.NotNil(tx)

	domain := testBuildAddress(require)
	store := &testCustodianStore{}

	count := 42
	custodian := testBuildAddress(require)
	tx.Extra = append(tx.Extra, custodian.PublicSpendKey[:]...)
	tx.Extra = append(tx.Extra, custodian.PublicViewKey[:]...)

	mainnet, _ := crypto.HashFromString(mainnetId)
	nodes := make([]*CustodianNode, count)
	for i := range count {
		signer := testBuildAddress(require)
		payee := testBuildAddress(require)
		custodian := testBuildAddress(require)
		extra := EncodeCustodianNode(&custodian, &payee, &signer.PrivateSpendKey, &payee.PrivateSpendKey, &custodian.PrivateSpendKey, mainnet)
		nodes[i] = &CustodianNode{custodian, payee, extra}
		tx.Extra = append(tx.Extra, extra...)
	}

	eh := crypto.Blake3Hash(tx.Extra)
	sig := domain.PrivateSpendKey.Sign(eh)
	tx.Extra = append(tx.Extra, sig[:]...)

	err := tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.Contains(err.Error(), "outputs count")

	random := testBuildAddress(require)
	amount := NewInteger(100).Mul(count - 1)
	tx.AddScriptOutput([]*Address{&random}, NewThresholdScript(Operator64), amount, bytes.Repeat([]byte{1}, 64))
	err = tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.Contains(err.Error(), "output type")

	tx.Outputs[0].Type = OutputTypeCustodianUpdateNodes
	err = tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.Contains(err.Error(), "sort order")

	sortedExtra := custodian.PublicSpendKey[:]
	sortedExtra = append(sortedExtra, custodian.PublicViewKey[:]...)
	sort.Slice(nodes, func(i, j int) bool {
		return bytes.Compare(nodes[i].Custodian.PublicSpendKey[:], nodes[j].Custodian.PublicSpendKey[:]) < 0
	})
	for _, n := range nodes {
		sortedExtra = append(sortedExtra, n.Extra...)
	}
	eh = crypto.Blake3Hash(sortedExtra)
	sig = domain.PrivateSpendKey.Sign(eh)
	tx.Extra = append(sortedExtra, sig[:]...)
	err = tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.Contains(err.Error(), "there must be a custodian")

	store.domain = &custodian
	err = tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.Contains(err.Error(), "approval signature")

	store.domain = &domain
	err = tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.Contains(err.Error(), "update price")

	tx.Outputs[0].Amount = NewInteger(100).Mul(count)
	err = tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.Nil(err)

	prev, err := store.ReadCustodian(uint64(time.Now().UnixNano()))
	require.Nil(err)
	require.NotNil(prev)
	require.Equal(domain.String(), prev.Custodian.String())

	timestamp := uint64(time.Now().UnixNano())
	store.custodianUpdateNodesTimestamp = timestamp
	store.custodianUpdateNodesExtra = tx.Extra
	store.custodianUpdateNodesTransaction = tx.AsVersioned().PayloadHash()

	prev, err = store.ReadCustodian(uint64(time.Now().UnixNano()))
	require.Nil(err)
	require.NotNil(prev)
	require.Equal(custodian.String(), prev.Custodian.String())
	require.Len(prev.Nodes, count)
	require.Equal(timestamp, prev.Timestamp)
	require.Equal(store.custodianUpdateNodesTransaction, prev.Transaction)

	err = tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.Contains(err.Error(), "approval signature")
	tx.Extra = tx.Extra[:len(tx.Extra)-64]
	eh = CustodianUpdateNodesApprovalHash(tx.Extra, prev.Transaction)
	sig = custodian.PrivateSpendKey.Sign(eh)
	tx.Extra = append(tx.Extra, sig[:]...)
	err = tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.Nil(err)

	tx.Outputs[0].Amount = NewInteger(1)
	err = tx.validateCustodianUpdateNodes(store, uint64(time.Now().UnixNano()))
	require.Nil(err)
}

func TestCustodianUpdateApprovalRejectsReplay(t *testing.T) {
	r := require.New(t)
	a, b := testBuildAddress(r), testBuildAddress(r)
	network := crypto.Blake3Hash([]byte("custodian approval replay test"))
	nodes := make([]*CustodianNode, custodianNodesMinimumCount)
	for i := range nodes {
		signer, payee, custodian := testBuildAddress(r), testBuildAddress(r), testBuildAddress(r)
		extra := EncodeCustodianNode(&custodian, &payee, &signer.PrivateSpendKey, &payee.PrivateSpendKey, &custodian.PrivateSpendKey, network)
		nodes[i] = &CustodianNode{Custodian: custodian, Payee: payee, Extra: extra}
	}
	sort.Slice(nodes, func(i, j int) bool {
		return bytes.Compare(nodes[i].Custodian.PublicSpendKey[:], nodes[j].Custodian.PublicSpendKey[:]) < 0
	})
	build := func(from, to Address, previous crypto.Hash, seed byte) *Transaction {
		tx := NewTransactionV5(XINAssetId)
		tx.AddScriptOutput([]*Address{&to}, NewThresholdScript(Operator64), NewInteger(1), bytes.Repeat([]byte{seed}, 64))
		tx.Outputs[0].Type = OutputTypeCustodianUpdateNodes
		tx.Extra = append(tx.Extra, to.PublicSpendKey[:]...)
		tx.Extra = append(tx.Extra, to.PublicViewKey[:]...)
		for _, n := range nodes {
			tx.Extra = append(tx.Extra, n.Extra...)
		}
		// Construct the preimage independently of the production hash helper.
		var preimage []byte
		if previous.HasValue() {
			preimage = append(preimage, previous[:]...)
		}
		preimage = append(preimage, tx.Extra...)
		sig := from.PrivateSpendKey.Sign(crypto.Blake3Hash(preimage))
		tx.Extra = append(tx.Extra, sig[:]...)
		return tx
	}
	for _, initial := range []struct {
		name string
		hash crypto.Hash
	}{
		{"no previous update", crypto.Hash{}},
		{"previous update", crypto.Blake3Hash([]byte("initial A update"))},
	} {
		t.Run(initial.name, func(t *testing.T) {
			r := require.New(t)
			store := &campaignStore{custodian: &CustodianUpdateRequest{Custodian: &a, Nodes: nodes, Transaction: initial.hash}}
			ab := build(a, b, initial.hash, 1)
			originalExtra := bytes.Clone(ab.Extra)
			r.NoError(ab.validateCustodianUpdateNodes(store, 1))
			r.Equal(originalExtra, ab.Extra, "validation must not modify extra")
			store.custodian = &CustodianUpdateRequest{Custodian: &b, Nodes: nodes, Transaction: ab.AsVersioned().PayloadHash()}
			ba := build(b, a, store.custodian.Transaction, 2)
			r.NoError(ba.validateCustodianUpdateNodes(store, 2))
			store.custodian = &CustodianUpdateRequest{Custodian: &a, Nodes: nodes, Transaction: ba.AsVersioned().PayloadHash()}

			// Copy the public A -> B approval into a fresh transaction after B -> A.
			replay := build(a, b, initial.hash, 3)
			replay.Extra = bytes.Clone(ab.Extra)
			replay.References = []crypto.Hash{store.custodian.Transaction}
			r.NotEqual(ab.AsVersioned().PayloadHash(), replay.AsVersioned().PayloadHash())
			r.Equal(ab.Extra, replay.Extra)
			r.ErrorContains(replay.validateCustodianUpdateNodes(store, 3), "approval signature")
			unbound := build(a, b, crypto.Hash{}, 4)
			r.ErrorContains(unbound.validateCustodianUpdateNodes(store, 3), "approval signature")
			fresh := build(a, b, store.custodian.Transaction, 5)
			r.NoError(fresh.validateCustodianUpdateNodes(store, 3))
		})
	}
}

func TestCustodianParseNode(t *testing.T) {
	require := require.New(t)

	mainnet, _ := crypto.HashFromString(mainnetId)
	payee := testBuildAddress(require)
	signer := testBuildAddress(require)
	custodian := testBuildAddress(require)
	nodeId := signer.Hash().ForNetwork(mainnet)
	extra := EncodeCustodianNode(&custodian, &payee, &signer.PrivateSpendKey, &payee.PrivateSpendKey, &custodian.PrivateSpendKey, mainnet)
	cn, err := parseCustodianNode(extra, false)
	require.Nil(err)
	require.NotNil(cn)
	require.Equal(custodian.String(), cn.Custodian.String())
	require.Equal(payee.String(), cn.Payee.String())
	require.Equal(extra, cn.Extra)
	require.Contains(hex.EncodeToString(extra), hex.EncodeToString(nodeId[:]))
	require.Nil(cn.validate())
}

type testCustodianStore struct {
	domain                          *Address
	custodianUpdateNodesExtra       []byte
	custodianUpdateNodesTimestamp   uint64
	custodianUpdateNodesTransaction crypto.Hash
}

func (s *testCustodianStore) ReadCustodian(_ uint64) (*CustodianUpdateRequest, error) {
	if s.custodianUpdateNodesExtra == nil {
		if s.domain == nil {
			return nil, nil
		}
		return &CustodianUpdateRequest{
			Custodian: s.domain,
		}, nil
	}
	cur, err := ParseCustodianUpdateNodesExtra(s.custodianUpdateNodesExtra, false)
	if err != nil {
		return nil, err
	}
	cur.Timestamp = s.custodianUpdateNodesTimestamp
	cur.Transaction = s.custodianUpdateNodesTransaction
	return cur, nil
}

func testBuildAddress(require *require.Assertions) Address {
	seed := make([]byte, 64)
	crypto.ReadRand(seed)
	addr := NewAddressFromSeed(seed)
	addr.PrivateViewKey = addr.PublicSpendKey.DeterministicHashDerive()
	addr.PublicViewKey = addr.PrivateViewKey.Public()
	return addr
}
