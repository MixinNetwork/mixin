package common

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestSignInputOrder(t *testing.T) {
	account := deterministicAddress(31)
	funding := NewTransactionV5(XINAssetId)
	funding.AddInput(crypto.Blake3Hash([]byte("funding")), 0)
	for i := range 2 {
		funding.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), NewInteger(1), bytes.Repeat([]byte{byte(32 + i)}, 64))
	}
	utxos := funding.AsVersioned().UnspentOutputs()
	store := &campaignStore{utxos: make(map[string]*UTXOWithLock)}
	for _, u := range utxos {
		store.utxos[utxoRef(u.Hash, u.Index)] = u
	}
	for _, order := range [][]int{{0, 1}, {1, 0}, {0, 0, 1}, {0, 1, 0}, {1, 0, 1}} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			tx := NewTransactionV5(XINAssetId)
			for _, u := range utxos {
				tx.AddInput(u.Hash, u.Index)
			}
			tx.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), NewInteger(2), bytes.Repeat([]byte{34}, 64))
			ver := tx.AsVersioned()
			for _, i := range order {
				require.NoError(t, ver.SignInput(store, i, []*Address{&account}))
			}
			require.Len(t, ver.SignaturesMap, len(tx.Inputs))
			require.NoError(t, ver.Validate(store, 1, false))
			decoded, err := UnmarshalVersionedTransaction(ver.Marshal())
			require.NoError(t, err)
			require.NoError(t, decoded.Validate(store, 1, false))
		})
	}
}

func TestSignInputRejectsNegativeIndex(t *testing.T) {
	account := deterministicAddress(35)
	tx := NewTransactionV5(XINAssetId)
	tx.AddInput(crypto.Blake3Hash([]byte("input")), 0)
	ver := tx.AsVersioned()
	require.ErrorContains(t, ver.SignInput(nil, -1, []*Address{&account}), "invalid input index")
	require.Empty(t, ver.SignaturesMap)
}

func TestSignUTXOOrder(t *testing.T) {
	account := deterministicAddress(38)
	funding := NewTransactionV5(XINAssetId)
	funding.AddInput(crypto.Blake3Hash([]byte("funding UTXOs")), 0)
	for i := range 2 {
		funding.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), NewInteger(1), bytes.Repeat([]byte{byte(39 + i)}, 64))
	}
	utxos := funding.AsVersioned().UnspentOutputs()
	store := &campaignStore{utxos: make(map[string]*UTXOWithLock)}
	for _, u := range utxos {
		store.utxos[utxoRef(u.Hash, u.Index)] = u
	}
	// The transaction input index differs from the referenced output index.
	utxos[0], utxos[1] = utxos[1], utxos[0]
	for _, order := range [][]int{{0, 1}, {1, 0}, {0, 0, 1}, {0, 1, 0}, {1, 0, 1}} {
		t.Run(fmt.Sprint(order), func(t *testing.T) {
			tx := NewTransactionV5(XINAssetId)
			for _, u := range utxos {
				tx.AddInput(u.Hash, u.Index)
			}
			tx.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), NewInteger(2), bytes.Repeat([]byte{41}, 64))
			ver := tx.AsVersioned()
			for _, i := range order {
				require.NoError(t, ver.SignUTXO(&utxos[i].UTXO, []*Address{&account}))
			}
			require.Len(t, ver.SignaturesMap, len(tx.Inputs))
			require.NoError(t, ver.Validate(store, 1, false))
			decoded, err := UnmarshalVersionedTransaction(ver.Marshal())
			require.NoError(t, err)
			require.NoError(t, decoded.Validate(store, 1, false))
		})
	}
}

func TestSignUTXORejectsUnreferencedOutput(t *testing.T) {
	account := deterministicAddress(42)
	hash := crypto.Blake3Hash([]byte("referenced input"))
	utxo, _ := makeScriptUTXO(account, hash, 2, NewInteger(1))
	for _, in := range []Input{
		{Hash: hash, Index: 1},
		{Hash: crypto.Blake3Hash([]byte("different input")), Index: 2},
	} {
		tx := NewTransactionV5(XINAssetId)
		tx.AddInput(in.Hash, in.Index)
		ver := tx.AsVersioned()
		require.ErrorContains(t, ver.SignUTXO(&utxo.UTXO, []*Address{&account}), "input not found")
		require.Empty(t, ver.SignaturesMap)
	}
}

func TestAggregateSignRejectsAccountCountMismatch(t *testing.T) {
	tx := NewTransactionV5(XINAssetId)
	tx.AddInput(crypto.Blake3Hash([]byte("first input")), 0)
	tx.AddInput(crypto.Blake3Hash([]byte("second input")), 0)
	for _, count := range []int{0, 1, 3} {
		ver := tx.AsVersioned()
		require.ErrorContains(t, ver.AggregateSign(nil, make([][]*Address, count), nil), "invalid signing accounts count")
		require.Nil(t, ver.AggregatedSignature)
	}
}

func TestSignRawReplacesExistingSignature(t *testing.T) {
	account := deterministicAddress(36)
	for _, mint := range []bool{false, true} {
		t.Run(fmt.Sprintf("mint=%t", mint), func(t *testing.T) {
			tx := NewTransactionV5(XINAssetId)
			if mint {
				tx.AddUniversalMintInput(1, NewInteger(1))
			} else {
				tx.AddDepositInput(&DepositData{Chain: BitcoinAssetId, AssetKey: "btc", Transaction: "deposit", Amount: NewInteger(1)})
			}
			tx.AddScriptOutput([]*Address{&account}, NewThresholdScript(1), NewInteger(1), bytes.Repeat([]byte{37}, 64))
			ver := tx.AsVersioned()
			for range 2 {
				require.NoError(t, ver.SignInput(nil, 0, []*Address{&account}))
			}
			require.Len(t, ver.SignaturesMap, 1)
			require.Len(t, ver.SignaturesMap[0], 1)
			require.True(t, account.PublicSpendKey.Verify(ver.PayloadHash(), *ver.SignaturesMap[0][0]))
		})
	}
}
