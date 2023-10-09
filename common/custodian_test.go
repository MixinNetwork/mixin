package common

import (
	"bytes"
	"crypto/rand"
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
	for i := 0; i < count; i++ {
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

	err := tx.validateCustodianUpdateNodes(store)
	require.NotNil(err)
	require.Contains(err.Error(), "outputs count")

	random := testBuildAddress(require)
	amount := NewInteger(100).Mul(count - 1)
	tx.AddScriptOutput([]*Address{&random}, NewThresholdScript(Operator64), amount, make([]byte, 64))
	err = tx.validateCustodianUpdateNodes(store)
	require.NotNil(err)
	require.Contains(err.Error(), "output type")

	tx.Outputs[0].Type = OutputTypeCustodianUpdateNodes
	err = tx.validateCustodianUpdateNodes(store)
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
	err = tx.validateCustodianUpdateNodes(store)
	require.NotNil(err)
	require.Contains(err.Error(), "there must be a custodian")

	store.domain = &custodian
	err = tx.validateCustodianUpdateNodes(store)
	require.NotNil(err)
	require.Contains(err.Error(), "approval signature")

	store.domain = &domain
	err = tx.validateCustodianUpdateNodes(store)
	require.NotNil(err)
	require.Contains(err.Error(), "update price")

	tx.Outputs[0].Amount = NewInteger(100).Mul(count)
	err = tx.validateCustodianUpdateNodes(store)
	require.Nil(err)

	prev, err := store.ReadCustodian(uint64(time.Now().UnixNano()))
	require.Nil(err)
	require.NotNil(prev)
	require.Equal(domain.String(), prev.Custodian.String())

	timestamp := uint64(time.Now().UnixNano())
	store.custodianUpdateNodesTimestamp = timestamp
	store.custodianUpdateNodesExtra = tx.Extra

	prev, err = store.ReadCustodian(uint64(time.Now().UnixNano()))
	require.Nil(err)
	require.NotNil(prev)
	require.Equal(custodian.String(), prev.Custodian.String())
	require.Len(prev.Nodes, count)
	require.Equal(timestamp, prev.Timestamp)

	err = tx.validateCustodianUpdateNodes(store)
	require.NotNil(err)
	require.Contains(err.Error(), "approval signature")
	tx.Extra = tx.Extra[:len(tx.Extra)-64]
	eh = crypto.Blake3Hash(tx.Extra)
	sig = custodian.PrivateSpendKey.Sign(eh)
	tx.Extra = append(tx.Extra, sig[:]...)
	err = tx.validateCustodianUpdateNodes(store)
	require.Nil(err)

	tx.Outputs[0].Amount = NewInteger(1)
	err = tx.validateCustodianUpdateNodes(store)
	require.Nil(err)
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
	domain                        *Address
	custodianUpdateNodesExtra     []byte
	custodianUpdateNodesTimestamp uint64
}

func (s *testCustodianStore) ReadCustodian(ts uint64) (*CustodianUpdateRequest, error) {
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
	return cur, nil
}

func testBuildAddress(require *require.Assertions) Address {
	seed := make([]byte, 64)
	n, err := rand.Read(seed)
	require.Nil(err)
	require.Equal(64, n)
	addr := NewAddressFromSeed(seed)
	addr.PrivateViewKey = addr.PublicSpendKey.DeterministicHashDerive()
	addr.PublicViewKey = addr.PrivateViewKey.Public()
	return addr
}
