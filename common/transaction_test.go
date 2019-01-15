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

	tx := NewTransaction(XINAssetId)
	tx.AddInput(genesisHash, 0)
	tx.AddInput(genesisHash, 1)
	tx.AddScriptOutput(accounts, script, NewInteger(20000))

	signed := &SignedTransaction{Transaction: *tx}
	for i, _ := range signed.Inputs {
		err := signed.SignInput(store, i, accounts)
		assert.Nil(err)
	}
	err := signed.Validate(store)
	assert.Nil(err)

	outputs := signed.ViewGhostKey(&accounts[1].PrivateViewKey)
	assert.Len(outputs, 1)
	assert.Equal(outputs[0].Keys[1].String(), accounts[1].PublicSpendKey.String())
	outputs = signed.ViewGhostKey(&accounts[1].PrivateSpendKey)
	assert.Len(outputs, 1)
	assert.NotEqual(outputs[0].Keys[1].String(), accounts[1].PublicSpendKey.String())
	assert.NotEqual(outputs[0].Keys[1].String(), accounts[1].PublicViewKey.String())
}

type storeImpl struct {
	seed     []byte
	accounts []Address
}

func (store storeImpl) SnapshotsReadUTXO(hash crypto.Hash, index int) (*UTXO, error) {
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
		key := crypto.DeriveGhostPublicKey(&genesisMaskr, &store.accounts[i].PublicViewKey, &store.accounts[i].PublicSpendKey)
		utxo.Keys = append(utxo.Keys, *key)
	}
	return utxo, nil
}

func (store storeImpl) SnapshotsCheckGhost(key crypto.Key) (bool, error) {
	return false, nil
}

func (store storeImpl) SnapshotsLockUTXO(hash crypto.Hash, index int, tx, snapHash crypto.Hash, ts uint64) (*UTXO, error) {
	return nil, nil
}

func (store storeImpl) SnapshotsReadAcceptedNodes() ([]Address, error) {
	return nil, nil
}

func randomAccount() Address {
	seed := make([]byte, 64)
	rand.Read(seed)
	return NewAddressFromSeed(seed)
}
