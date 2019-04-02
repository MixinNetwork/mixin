package common

import (
	"crypto/rand"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestTransaction(t *testing.T) {
	assert := assert.New(t)

	accounts := make([]Address, 0)
	for i := 0; i < 3; i++ {
		accounts = append(accounts, randomAccount())
	}

	seed := make([]byte, 64)
	rand.Read(seed)
	genesisHash := crypto.Hash{}
	script := Script{OperatorCmp, OperatorSum, 2}
	store := storeImpl{seed: seed, accounts: accounts}

	ver := NewTransaction(XINAssetId).AsLatestVersion()
	assert.Equal("d2cf4d6e85d76512b29f173073be167423705e207f090f8cfc3e2b61fc32b6e2", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 0)
	ver.AddInput(genesisHash, 1)
	assert.Equal("e31ea7bd97a59169fbef1294b4dcc00dd33b6c4cd95367614415a5d6bdb1eee8", ver.PayloadHash().String())
	ver.AddScriptOutput(accounts, script, NewInteger(20000))

	for i, _ := range ver.Inputs {
		err := ver.SignInput(store, i, accounts)
		assert.Nil(err)
	}
	err := ver.Validate(store)
	assert.Nil(err)

	outputs := ver.ViewGhostKey(&accounts[1].PrivateViewKey)
	assert.Len(outputs, 1)
	assert.Equal(outputs[0].Keys[1].String(), accounts[1].PublicSpendKey.String())
	outputs = ver.ViewGhostKey(&accounts[1].PrivateSpendKey)
	assert.Len(outputs, 1)
	assert.NotEqual(outputs[0].Keys[1].String(), accounts[1].PublicSpendKey.String())
	assert.NotEqual(outputs[0].Keys[1].String(), accounts[1].PublicViewKey.String())
}

type storeImpl struct {
	seed     []byte
	accounts []Address
}

func (store storeImpl) ReadUTXO(hash crypto.Hash, index int) (*UTXO, error) {
	genesisMaskr := crypto.NewKeyFromSeed(store.seed)
	genesisMaskR := genesisMaskr.Public()

	in := Input{
		Hash:  hash,
		Index: index,
	}
	out := Output{
		Type:   OutputTypeScript,
		Amount: NewInteger(10000),
		Script: Script{OperatorCmp, OperatorSum, uint8(index + 1)},
		Mask:   genesisMaskR,
	}
	utxo := &UTXO{
		Input:  in,
		Output: out,
		Asset:  XINAssetId,
	}

	for i := 0; i <= index; i++ {
		key := crypto.DeriveGhostPublicKey(&genesisMaskr, &store.accounts[i].PublicViewKey, &store.accounts[i].PublicSpendKey, uint64(index))
		utxo.Keys = append(utxo.Keys, *key)
	}
	return utxo, nil
}

func (store storeImpl) CheckGhost(key crypto.Key) (bool, error) {
	return false, nil
}

func (store storeImpl) LockUTXO(hash crypto.Hash, index int, tx crypto.Hash, fork bool) (*UTXO, error) {
	return nil, nil
}

func (store storeImpl) ReadDomains() []Domain {
	return nil
}

func (store storeImpl) ReadConsensusNodes() []*Node {
	return nil
}

func (store storeImpl) ReadTransaction(hash crypto.Hash) (*VersionedTransaction, error) {
	return nil, nil
}

func (store storeImpl) CheckDepositInput(deposit *DepositData, tx crypto.Hash) error {
	return nil
}

func (store storeImpl) LockDepositInput(deposit *DepositData, tx crypto.Hash, fork bool) error {
	return nil
}

func (store storeImpl) ReadLastMintDistribution(group string) (*MintDistribution, error) {
	return nil, nil
}

func (store storeImpl) LockMintInput(mint *MintData, tx crypto.Hash, fork bool) error {
	return nil
}

func randomAccount() Address {
	seed := make([]byte, 64)
	rand.Read(seed)
	return NewAddressFromSeed(seed)
}
