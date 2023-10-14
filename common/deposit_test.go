package common

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestDeposit(t *testing.T) {
	require := require.New(t)

	accounts := make([]*Address, 0)
	for i := 0; i < 16; i++ {
		seed := make([]byte, 64)
		seed[i] = byte(i)
		a := NewAddressFromSeed(seed)
		accounts = append(accounts, &a)
	}

	chainId, _ := crypto.HashFromString("8dd50817c082cdcdd6f167514928767a4b52426997bd6d4930eca101c5ff8a27")
	assetKey := "0xa974c709cfb4566686553a20790685a47aceaa33"
	assetID, _ := crypto.HashFromString("a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc")
	transactionHash := "0x426ce53523b2f24d0f20707ef169f9cc5a1eea34287210873421bd1e5e5d2718"

	receiver := &MixinKey{
		ViewKey:  "981ec8403e35b3feb829a7734b8cf56a1229bb344f59fa2766453aa17e931f02",
		SpendKey: "c8327d02a2b79c0f15f8d70118836a79b88d9942cabaaa2b90486a49ec07b001",
	}

	sender := &MixinKey{
		ViewKey:  "77ac6731865c29247588b14dff8e163c81dfaac130cc22882b77a0539db00b0f",
		SpendKey: "87be1eeb3b72909b5447a1699af7538fc0a492222d7b8ab98187299adc4d1b0e",
	}

	seed := make([]byte, 64)
	crypto.ReadRand(seed)
	store := storeImpl{
		custodian: receiver.Address(),
		seed:      seed,
		accounts:  accounts,
	}

	tx := NewTransactionV5(assetID)
	ver := tx.AsVersioned()
	signed := &ver.SignedTransaction
	err := signed.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.True(strings.Contains(err.Error(), "invalid inputs"))

	tx.AddDepositInput(&DepositData{
		Chain:       chainId,
		AssetKey:    assetKey,
		Transaction: transactionHash,
		Index:       0,
		Amount:      NewIntegerFromString("1006"),
	})
	tx.AddDepositInput(&DepositData{
		Chain:       chainId,
		AssetKey:    assetKey,
		Transaction: transactionHash,
		Index:       1,
		Amount:      NewIntegerFromString("1007"),
	})
	ver = tx.AsVersioned()
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.True(strings.Contains(err.Error(), "invalid inputs"))

	tx = NewTransactionV5(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:       chainId,
		AssetKey:    assetKey,
		Transaction: transactionHash,
		Index:       0,
		Amount:      NewIntegerFromString("1006"),
	})
	ver = tx.AsVersioned()
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.True(strings.Contains(err.Error(), "invalid outputs"))

	tx = NewTransactionV5(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:       chainId,
		AssetKey:    assetKey,
		Transaction: transactionHash,
		Index:       0,
		Amount:      NewIntegerFromString("1006"),
	})
	si := crypto.Blake3Hash([]byte(transactionHash + "1"))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address()}, NewThresholdScript(1), NewIntegerFromString("1006"), seed)
	si = crypto.Blake3Hash([]byte(transactionHash + "2"))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address()}, NewThresholdScript(1), NewIntegerFromString("1007"), seed)
	ver = tx.AsVersioned()
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.True(strings.Contains(err.Error(), "invalid outputs"))

	tx = NewTransactionV5(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:       chainId,
		AssetKey:    assetKey,
		Transaction: transactionHash,
		Index:       0,
		Amount:      NewIntegerFromString("1006"),
	})
	si = crypto.Blake3Hash([]byte(transactionHash))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address()}, NewThresholdScript(1), NewIntegerFromString("1006"), seed)
	ver = tx.AsVersioned()
	domain := parseKeyFromHex(sender.SpendKey)
	err = ver.SignRaw(domain)
	require.Nil(err)
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, uint64(time.Now().UnixNano()))
	require.NotNil(err)
	require.True(strings.Contains(err.Error(), "invalid domain signature for deposit"))

	tx = NewTransactionV5(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:       chainId,
		AssetKey:    assetKey,
		Transaction: transactionHash,
		Index:       0,
		Amount:      NewIntegerFromString("1006"),
	})
	si = crypto.Blake3Hash([]byte(transactionHash))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address()}, NewThresholdScript(1), NewIntegerFromString("1006"), seed)
	ver = tx.AsVersioned()
	domain = parseKeyFromHex(receiver.SpendKey)
	err = ver.SignRaw(domain)
	require.Nil(err)
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, uint64(time.Now().UnixNano()))
	require.Nil(err)

	tx = NewTransactionV5(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:       chainId,
		AssetKey:    assetKey,
		Transaction: transactionHash,
		Index:       0,
		Amount:      NewIntegerFromString("1006"),
	})
	si = crypto.Blake3Hash([]byte(transactionHash))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address(), sender.Address()}, NewThresholdScript(2), NewIntegerFromString("1006"), seed)
	ver = tx.AsVersioned()
	domain = parseKeyFromHex(receiver.SpendKey)
	err = ver.SignRaw(domain)
	require.Nil(err)
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, uint64(time.Now().UnixNano()))
	require.Nil(err)

	tx = NewTransactionV5(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:       chainId,
		AssetKey:    assetKey,
		Transaction: transactionHash,
		Index:       0,
		Amount:      NewIntegerFromString("1006"),
	})
	si = crypto.Blake3Hash([]byte(transactionHash))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address(), sender.Address()}, NewThresholdScript(1), NewIntegerFromString("1006"), seed)
	ver = tx.AsVersioned()
	domain = parseKeyFromHex(receiver.SpendKey)
	err = ver.SignRaw(domain)
	require.Nil(err)
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, uint64(time.Now().UnixNano()))
	require.Nil(err)

	pm := ver.Marshal()
	ver, err = UnmarshalVersionedTransaction(pm)
	require.Nil(err)
	require.Equal(1, len(ver.SignaturesMap))
	require.Equal(1, len(ver.SignaturesMap[0]))
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, ver.PayloadHash(), ver.SignaturesMap, uint64(time.Now().UnixNano()))
	require.Nil(err)
}

type MixinKey struct {
	ViewKey  string
	SpendKey string
}

func (mk *MixinKey) Address() *Address {
	a := Address{
		PrivateViewKey:  parseKeyFromHex(mk.ViewKey),
		PrivateSpendKey: parseKeyFromHex(mk.SpendKey),
	}
	a.PublicViewKey = a.PrivateViewKey.Public()
	a.PublicSpendKey = a.PrivateSpendKey.Public()
	return &a
}

func parseKeyFromHex(src string) crypto.Key {
	var key crypto.Key
	data, _ := hex.DecodeString(src)
	copy(key[:], data)
	return key
}
