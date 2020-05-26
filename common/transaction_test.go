// +build ed25519 !custom_alg

package common

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestTransaction(t *testing.T) {
	assert := assert.New(t)

	accounts := make([]Address, 0)
	for i := 0; i < 3; i++ {
		seed := make([]byte, 64)
		seed[i] = byte(i)
		accounts = append(accounts, NewAddressFromSeed(seed))
	}

	seed := make([]byte, 64)
	rand.Read(seed)
	genesisHash := crypto.Hash{}
	script := Script{OperatorCmp, OperatorSum, 2}
	store := storeImpl{seed: seed, accounts: accounts}

	ver := NewTransaction(XINAssetId).AsLatestVersion()
	assert.Equal("d2cf4d6e85d76512b29f173073be167423705e207f090f8cfc3e2b61fc32b6e2", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 0)
	assert.Equal("b3afe7497740e05ba83e26977fbbfe7e1c2efc312d8d9aeb93bce43b9d8c6248", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 1)
	assert.Equal("e31ea7bd97a59169fbef1294b4dcc00dd33b6c4cd95367614415a5d6bdb1eee8", ver.PayloadHash().String())
	ver.Outputs = append(ver.Outputs, &Output{Type: OutputTypeScript, Amount: NewInteger(10000), Script: script, Mask: crypto.NewPrivateKeyFromSeed(bytes.Repeat([]byte{1}, 64)).Key()})
	assert.Equal("56fb588ab4319a54694fbbdc85f41b913401137da83ac6724e2c3adb076460f9", ver.PayloadHash().String())
	ver.AddScriptOutput(accounts, script, NewInteger(10000), bytes.Repeat([]byte{1}, 64))
	assert.Equal("d0a26a0a7f05941bc748b8f605f0b990511aafb865cf759364eb1d46156e6696", ver.PayloadHash().String())

	pm := ver.Marshal()
	assert.Equal(488, len(pm))
	assert.Equal("86a756657273696f6e01a54173736574c420a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdca6496e707574739285a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c085a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657801a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739285a45479706500a6416d6f756e74c70500e8d4a51000a44b657973c0a6536372697074c403fffe02a44d61736bc4204fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580585a45479706500a6416d6f756e74c70500e8d4a51000a44b65797393c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68a6536372697074c403fffe02a44d61736bc420c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ea54578747261c0aa5369676e617475726573c0", hex.EncodeToString(pm))
	cm := ver.CompressMarshal()
	assert.Equal(277, len(cm))
	assert.Equal("0000000028b52ffd63c118533ce8001d0800c40ca99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc921000024fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580593c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ec012fc75bb1394a785cc89cb470d24f85aedd083bc20068f76facb1fb825e9b432c19f5bf7a53b2c3bc0e7ce5e6fd9f8cbae17a5ec0e", hex.EncodeToString(cm))
	ver, err := DecompressUnmarshalVersionedTransaction(cm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(488, len(pm))
	assert.Equal("86a756657273696f6e01a54173736574c420a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdca6496e707574739285a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c085a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657801a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739285a45479706500a6416d6f756e74c70500e8d4a51000a44b657973c0a6536372697074c403fffe02a44d61736bc4204fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580585a45479706500a6416d6f756e74c70500e8d4a51000a44b65797393c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68a6536372697074c403fffe02a44d61736bc420c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ea54578747261c0aa5369676e617475726573c0", hex.EncodeToString(pm))
	ver, err = DecompressUnmarshalVersionedTransaction(pm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(488, len(pm))
	assert.Equal("86a756657273696f6e01a54173736574c420a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdca6496e707574739285a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c085a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657801a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739285a45479706500a6416d6f756e74c70500e8d4a51000a44b657973c0a6536372697074c403fffe02a44d61736bc4204fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580585a45479706500a6416d6f756e74c70500e8d4a51000a44b65797393c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68a6536372697074c403fffe02a44d61736bc420c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ea54578747261c0aa5369676e617475726573c0", hex.EncodeToString(pm))
	cm, err = hex.DecodeString("0000000028b52ffd63c118533ce8000d0800c40ca99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc921000024fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580593c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ec012fc75bb1394a785cc89cb470d24f85aedd083bc20068f76facb1fb825e9b412fc6f5f9ceeb02cf873f76adfc66b76ed3c6577")
	assert.Nil(err)
	ver, err = DecompressUnmarshalVersionedTransaction(cm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(488, len(pm))
	assert.Equal("86a756657273696f6e01a54173736574c420a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdca6496e707574739285a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c085a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657801a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739285a45479706500a6416d6f756e74c70500e8d4a51000a44b657973c0a6536372697074c403fffe02a44d61736bc4204fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580585a45479706500a6416d6f756e74c70500e8d4a51000a44b65797393c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68a6536372697074c403fffe02a44d61736bc420c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ea54578747261c0aa5369676e617475726573c0", hex.EncodeToString(pm))
	cm, err = hex.DecodeString("0000000028b52ffd63c118533ce8001d0800c40ca99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc921000024fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580593c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ec012fc75bb1394a785cc89cb470d24f85aedd083bc20068f76facb1fb825e9b432c19f5bf7a53b2c3bc0e7ce5e6fd9f8cbae17a5ec0e")
	assert.Nil(err)
	ver, err = DecompressUnmarshalVersionedTransaction(cm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(488, len(pm))
	assert.Equal("86a756657273696f6e01a54173736574c420a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdca6496e707574739285a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c085a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657801a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739285a45479706500a6416d6f756e74c70500e8d4a51000a44b657973c0a6536372697074c403fffe02a44d61736bc4204fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580585a45479706500a6416d6f756e74c70500e8d4a51000a44b65797393c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68a6536372697074c403fffe02a44d61736bc420c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ea54578747261c0aa5369676e617475726573c0", hex.EncodeToString(pm))

	for i := range ver.Inputs {
		err := ver.SignInput(store, i, accounts)
		assert.NotNil(err)
		assert.Contains(err.Error(), "invalid key for the input")
	}
	err = ver.Validate(store)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid tx signature number")

	for i := range ver.Inputs {
		err := ver.SignInput(store, i, accounts[0:i+1])
		assert.Nil(err)
	}
	err = ver.Validate(store)
	assert.Nil(err)

	outputs := ver.ViewGhostKey(accounts[1].PrivateViewKey)
	assert.Len(outputs, 2)
	assert.Equal(outputs[1].Keys[1].String(), accounts[1].PublicSpendKey.String())
	outputs = ver.ViewGhostKey(accounts[1].PrivateSpendKey)
	assert.Len(outputs, 2)
	assert.NotEqual(outputs[1].Keys[1].String(), accounts[1].PublicSpendKey.String())
	assert.NotEqual(outputs[1].Keys[1].String(), accounts[1].PublicViewKey.String())
}

type storeImpl struct {
	seed     []byte
	accounts []Address
}

func (store storeImpl) ReadUTXO(hash crypto.Hash, index int) (*UTXOWithLock, error) {
	genesisMaskr := crypto.NewPrivateKeyFromSeed(store.seed)
	genesisMaskR := genesisMaskr.Public()

	in := Input{
		Hash:  hash,
		Index: index,
	}
	out := Output{
		Type:   OutputTypeScript,
		Amount: NewInteger(10000),
		Script: Script{OperatorCmp, OperatorSum, uint8(index + 1)},
		Mask:   genesisMaskR.Key(),
	}
	utxo := &UTXOWithLock{
		UTXO: UTXO{
			Input:  in,
			Output: out,
			Asset:  XINAssetId,
		},
	}

	for i := 0; i <= index; i++ {
		key := crypto.DeriveGhostPublicKey(genesisMaskr, store.accounts[i].PublicViewKey, store.accounts[i].PublicSpendKey, uint64(index)).Key()
		utxo.Keys = append(utxo.Keys, key)
	}
	return utxo, nil
}

func (store storeImpl) CheckGhost(key crypto.Key) (bool, error) {
	return false, nil
}

func (store storeImpl) LockUTXO(hash crypto.Hash, index int, tx crypto.Hash, fork bool) error {
	return nil
}

func (store storeImpl) ReadDomains() []Domain {
	return nil
}

func (store storeImpl) ReadAllNodes() []*Node {
	return nil
}

func (store storeImpl) ReadConsensusNodes() []*Node {
	return nil
}

func (store storeImpl) ReadTransaction(hash crypto.Hash) (*VersionedTransaction, string, error) {
	return nil, "", nil
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

func (store storeImpl) LockWithdrawalClaim(hash, tx crypto.Hash, fork bool) error {
	return nil
}
