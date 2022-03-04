package common

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/ethereum"
	"github.com/stretchr/testify/assert"
)

func TestDeposit(t *testing.T) {
	assert := assert.New(t)

	accounts := make([]*Address, 0)
	for i := 0; i < 16; i++ {
		seed := make([]byte, 64)
		seed[i] = byte(i)
		a := NewAddressFromSeed(seed)
		accounts = append(accounts, &a)
	}

	assetKey := "0xa974c709cfb4566686553a20790685a47aceaa33"
	assetID := ethereum.GenerateAssetId(assetKey)
	transactionHash := "0x426ce53523b2f24d0f20707ef169f9cc5a1eea34287210873421bd1e5e5d2718"
	viewKey := "568302b687a2fa3e8853ff35d99ffdf3817b98170de7b51e43d0dcf4fe30470f"
	requestID := "d771401f-fa27-413e-84c3-af1edc57ae17"

	receiver := &MixinKey{
		UserId:   "477c8d28-3060-3e11-a278-802f2c37f815",
		ViewKey:  "981ec8403e35b3feb829a7734b8cf56a1229bb344f59fa2766453aa17e931f02",
		SpendKey: "c8327d02a2b79c0f15f8d70118836a79b88d9942cabaaa2b90486a49ec07b001",
	}

	sender := &MixinKey{
		UserId:   "2b9a8ab4-dc66-3956-9356-0c31963d56f9",
		ViewKey:  "77ac6731865c29247588b14dff8e163c81dfaac130cc22882b77a0539db00b0f",
		SpendKey: "87be1eeb3b72909b5447a1699af7538fc0a492222d7b8ab98187299adc4d1b0e",
	}

	seed := make([]byte, 64)
	rand.Read(seed)
	store := storeImpl{
		seed:     seed,
		accounts: accounts,
		domains: []Domain{
			Domain{
				Account: *receiver.Address(),
			},
		},
	}

	tx := NewTransaction(assetID)
	ver := tx.AsLatestVersion()
	msg := ver.PayloadMarshal()
	signed := &ver.SignedTransaction
	err := signed.validateDeposit(store, msg, ver.PayloadHash(), ver.SignaturesMap)
	assert.NotNil(err)
	assert.True(strings.Contains(err.Error(), "invalid inputs"))

	tx.AddDepositInput(&DepositData{
		Chain:           ethereum.EthereumChainId,
		AssetKey:        assetKey,
		TransactionHash: transactionHash,
		OutputIndex:     0,
		Amount:          NewIntegerFromString("1006"),
	})
	tx.AddDepositInput(&DepositData{
		Chain:           ethereum.EthereumChainId,
		AssetKey:        assetKey,
		TransactionHash: transactionHash,
		OutputIndex:     1,
		Amount:          NewIntegerFromString("1007"),
	})
	ver = tx.AsLatestVersion()
	msg = ver.PayloadMarshal()
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, msg, ver.PayloadHash(), ver.SignaturesMap)
	assert.NotNil(err)
	assert.True(strings.Contains(err.Error(), "invalid inputs"))

	tx = NewTransaction(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:           ethereum.EthereumChainId,
		AssetKey:        assetKey,
		TransactionHash: transactionHash,
		OutputIndex:     0,
		Amount:          NewIntegerFromString("1006"),
	})
	ver = tx.AsLatestVersion()
	msg = ver.PayloadMarshal()
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, msg, ver.PayloadHash(), ver.SignaturesMap)
	assert.NotNil(err)
	assert.True(strings.Contains(err.Error(), "invalid outputs"))

	tx = NewTransaction(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:           ethereum.EthereumChainId,
		AssetKey:        assetKey,
		TransactionHash: transactionHash,
		OutputIndex:     0,
		Amount:          NewIntegerFromString("1006"),
	})
	si := crypto.NewHash([]byte("DEPOSIT" + viewKey + requestID))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address()}, NewThresholdScript(1), NewIntegerFromString("1006"), seed)
	si = crypto.NewHash([]byte("DEPOSIT" + viewKey + "7d7d5c8e-8f3e-44db-8d0d-beed5e739469"))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address()}, NewThresholdScript(1), NewIntegerFromString("1007"), seed)
	ver = tx.AsLatestVersion()
	msg = ver.PayloadMarshal()
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, msg, ver.PayloadHash(), ver.SignaturesMap)
	assert.NotNil(err)
	assert.True(strings.Contains(err.Error(), "invalid outputs"))

	tx = NewTransaction(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:           ethereum.EthereumChainId,
		AssetKey:        assetKey,
		TransactionHash: transactionHash,
		OutputIndex:     0,
		Amount:          NewIntegerFromString("1006"),
	})
	si = crypto.NewHash([]byte("DEPOSIT" + viewKey + requestID))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address()}, NewThresholdScript(1), NewIntegerFromString("1006"), seed)
	ver = tx.AsLatestVersion()
	domain := parseKeyFromHex(receiver.SpendKey)
	err = ver.SignRaw(domain)
	assert.Nil(err)
	msg = ver.PayloadMarshal()
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, msg, ver.PayloadHash(), ver.SignaturesMap)
	assert.Nil(err)

	tx = NewTransaction(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:           ethereum.EthereumChainId,
		AssetKey:        assetKey,
		TransactionHash: transactionHash,
		OutputIndex:     0,
		Amount:          NewIntegerFromString("1006"),
	})
	si = crypto.NewHash([]byte("DEPOSIT" + viewKey + requestID))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address(), sender.Address()}, NewThresholdScript(2), NewIntegerFromString("1006"), seed)
	ver = tx.AsLatestVersion()
	domain = parseKeyFromHex(receiver.SpendKey)
	err = ver.SignRaw(domain)
	assert.Nil(err)
	msg = ver.PayloadMarshal()
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, msg, ver.PayloadHash(), ver.SignaturesMap)
	assert.Nil(err)

	tx = NewTransaction(assetID)
	tx.AddDepositInput(&DepositData{
		Chain:           ethereum.EthereumChainId,
		AssetKey:        assetKey,
		TransactionHash: transactionHash,
		OutputIndex:     0,
		Amount:          NewIntegerFromString("1006"),
	})
	si = crypto.NewHash([]byte("DEPOSIT" + viewKey + requestID))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*Address{receiver.Address(), sender.Address()}, NewThresholdScript(1), NewIntegerFromString("1006"), seed)
	ver = tx.AsLatestVersion()
	domain = parseKeyFromHex(receiver.SpendKey)
	err = ver.SignRaw(domain)
	assert.Nil(err)
	msg = ver.PayloadMarshal()
	signed = &ver.SignedTransaction
	err = signed.validateDeposit(store, msg, ver.PayloadHash(), ver.SignaturesMap)
	assert.Nil(err)
}
