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

	PM := "77770002a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc00020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000200000005e8d4a5100000004fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f258050003fffe0d000000000005e8d4a51000001082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fdf02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb6832218c3a5ac643ff812bf9968fa545ea3862e8c103762e0eef25c4969ddb1cf262e1678c55a525f1be99c3168fd0d9e5aa4058046a0dace30c0eacca6570f976bb5214f113d3c99bf80c7336f9ce4a15af88e782cb3b912162db7c94a93ef12ffed7db88dbb7f9eb9b4ffd36493551ab1aecabc6d1153c9e5ce62599cfe68a28470d974e6e1397a055175082a606916d10becc943e01c39c1f40cf784d016ab28bc8c3e483b06ea5abb6c7f1f55683b903071205ed0c8d0a7079b647fdd8f49784d74d969eded1ab4fea0c98515bad32fbb7587a13de9e64f7ffd0d7b7d3c358867d3ece1fd8e73df21402b0585a359503ae28d5e57aaa47918a70fc2fe2c73855a3baacb8acf8e87830f70b28737cd91d6b733681da009d0d7a69de93eff57cfa973a8156c81379bf470c83a1c64dbb05e3dd060d87575dcc3b0b40d75b06719ef8473ab7400748532e593bd84405390b50ca0ef514b7a75bc74d9632183a4de891a54b45813fd35c739402dc1321c43da131722dff4befd6cfcaaa73cfa8054623dd0c98361eb656e5d9dfd6ec5332fa323f973e1693645fb7d06843898b91c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53e0003fffe0d000000000000"
	CM := "0000000028b52ffd63c118533ce001751400842777770002a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc000200010005e8d4a5100000004fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f258050003fffe0d1082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fdf02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb6832218c3a5ac643ff812bf9968fa545ea3862e8c103762e0eef25c4969ddb1cf262e1678c55a525f1be99c3168fd0d9e5aa4058046a0dace30c0eacca6570f976bb5214f113d3c99bf80c7336f9ce4a15af88e782cb3b912162db7c94a93ef12ffed7db88dbb7f9eb9b4ffd36493551ab1aecabc6d1153c9e5ce62599cfe68a28470d974e6e1397a055175082a606916d10becc943e01c39c1f40cf784d016ab28bc8c3e483b06ea5abb6c7f1f55683b903071205ed0c8d0a7079b647fdd8f49784d74d969eded1ab4fea0c98515bad32fbb7587a13de9e64f7ffd0d7b7d3c358867d3ece1fd8e73df21402b0585a359503ae28d5e57aaa47918a70fc2fe2c73855a3baacb8acf8e87830f70b28737cd91d6b733681da009d0d7a69de93eff57cfa973a8156c81379bf470c83a1c64dbb05e3dd060d87575dcc3b0b40d75b06719ef8473ab7400748532e593bd84405390b50ca0ef514b7a75bc74d9632183a4de891a54b45813fd35c739402dc1321c43da131722dff4befd6cfcaaa73cfa8054623dd0c98361eb656e5d9dfd6ec5332fa323f973e1693645fb7d06843898b91c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53e0006fc216adc5f5c5f62d1b5feaa7815c6eb86819d"

	accounts := make([]*Address, 0)
	for i := 0; i < 16; i++ {
		seed := make([]byte, 64)
		seed[i] = byte(i)
		a := NewAddressFromSeed(seed)
		accounts = append(accounts, &a)
	}

	seed := make([]byte, 64)
	rand.Read(seed)
	genesisHash := crypto.Hash{}
	script := Script{OperatorCmp, OperatorSum, 13}
	store := storeImpl{seed: seed, accounts: accounts}

	ver := NewTransaction(XINAssetId).AsLatestVersion()
	assert.Equal("b2d01ebf49e4a16f405c72a51fc949554ec7bd7ad99d9581bd807cc83157f126", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 0)
	ver.resetCache()
	assert.Equal("66b277cb31c9f409dee65c81abfd0c8034c8a1067e04e27137daf69c404bf95b", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 1)
	ver.resetCache()
	assert.Equal("a0d1ab5b23f5f81d4f91b160f60c9d05766462c11fe52a39459b09aa77630530", ver.PayloadHash().String())
	ver.Outputs = append(ver.Outputs, &Output{Type: OutputTypeScript, Amount: NewInteger(10000), Script: script, Mask: crypto.NewKeyFromSeed(bytes.Repeat([]byte{1}, 64))})
	ver.resetCache()
	assert.Equal("b1d57d9b3f15ad2a8fd143a76c4c7008579f7613a27ef7f2f626146cbe8dfdb7", ver.PayloadHash().String())
	ver.AddScriptOutput(accounts, script, NewInteger(10000), bytes.Repeat([]byte{1}, 64))
	ver.resetCache()
	assert.Equal("ab4101ecd51e82fe37bfffcf3177f9cbe9b4eda8ba4ccd50acba149eb700db7b", ver.PayloadHash().String())

	pm := ver.Marshal()
	assert.Equal(736, len(pm))
	assert.Equal(PM, hex.EncodeToString(pm))
	cm := ver.CompressMarshal()
	assert.Equal(672, len(cm))
	assert.Equal(CM, hex.EncodeToString(cm))
	ver, err := DecompressUnmarshalVersionedTransaction(cm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(736, len(pm))
	assert.Equal(PM, hex.EncodeToString(pm))
	ver, err = DecompressUnmarshalVersionedTransaction(pm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(736, len(pm))
	assert.Equal(PM, hex.EncodeToString(pm))
	cm, err = hex.DecodeString(CM)
	assert.Nil(err)
	ver, err = DecompressUnmarshalVersionedTransaction(cm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(736, len(pm))
	assert.Equal(PM, hex.EncodeToString(pm))

	for i := range ver.Inputs {
		err := ver.SignInput(store, i, accounts)
		assert.NotNil(err)
		assert.Contains(err.Error(), "invalid key for the input")
	}
	err = ver.Validate(store)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid tx signature number")

	ver.SignaturesMap = nil
	for i := range ver.Inputs {
		err := ver.SignInput(store, i, accounts[0:i+1])
		assert.Nil(err)
		err = ver.Validate(store)
		if i < len(ver.Inputs)-1 {
			assert.NotNil(err)
		} else {
			assert.Nil(err)
		}
	}
	err = ver.Validate(store)
	assert.Nil(err)

	assert.Len(ver.Inputs, 2)
	assert.Len(ver.SignaturesSliceV1, 0)
	assert.Len(ver.SignaturesMap, 2)
	assert.Len(ver.SignaturesMap[0], 1)
	assert.Len(ver.SignaturesMap[1], 2)
	om := ver.SignaturesMap
	sm := make([]map[uint16]*crypto.Signature, 2)
	for i, m := range om {
		if sm[i] == nil {
			sm[i] = make(map[uint16]*crypto.Signature)
		}
		for j, s := range m {
			sm[i][j+1] = s
		}
	}
	ver.SignaturesMap = sm
	err = ver.Validate(store)
	assert.NotNil(err)
	assert.Equal("batch verification failure 3 3", err.Error())
	sm = make([]map[uint16]*crypto.Signature, 2)
	for i, m := range om {
		if sm[i] == nil {
			sm[i] = make(map[uint16]*crypto.Signature)
		}
		for j, s := range m {
			sm[i][j+2] = s
		}
	}
	ver.SignaturesMap = sm
	err = ver.Validate(store)
	assert.NotNil(err)
	assert.Equal("invalid signature map index 2 2", err.Error())
	sm = make([]map[uint16]*crypto.Signature, 2)
	for i, m := range om {
		if sm[i] == nil {
			sm[i] = make(map[uint16]*crypto.Signature)
		}
		for j, s := range m {
			sm[i][j] = s
		}
	}
	sm[0][1] = sm[0][0]
	ver.SignaturesMap = sm
	err = ver.Validate(store)
	assert.NotNil(err)
	assert.Equal("batch verification failure 4 4", err.Error())
	sm = make([]map[uint16]*crypto.Signature, 2)
	for i, m := range om {
		if sm[i] == nil {
			sm[i] = make(map[uint16]*crypto.Signature)
		}
		for j, s := range m {
			sm[i][j] = s
		}
	}
	sm[1][0] = sm[0][0]
	ver.SignaturesMap = sm
	err = ver.Validate(store)
	assert.NotNil(err)
	assert.Equal("batch verification failure 3 3", err.Error())

	outputs := ver.ViewGhostKey(&accounts[1].PrivateViewKey)
	assert.Len(outputs, 2)
	assert.Equal(outputs[1].Keys[1].String(), accounts[1].PublicSpendKey.String())
	outputs = ver.ViewGhostKey(&accounts[1].PrivateSpendKey)
	assert.Len(outputs, 2)
	assert.NotEqual(outputs[1].Keys[1].String(), accounts[1].PublicSpendKey.String())
	assert.NotEqual(outputs[1].Keys[1].String(), accounts[1].PublicViewKey.String())
}

type storeImpl struct {
	seed     []byte
	accounts []*Address
}

func (store storeImpl) ReadUTXO(hash crypto.Hash, index int) (*UTXOWithLock, error) {
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
	utxo := &UTXOWithLock{
		UTXO: UTXO{
			Input:  in,
			Output: out,
			Asset:  XINAssetId,
		},
	}

	for i := 0; i <= index+1; i++ {
		key := crypto.DeriveGhostPublicKey(&genesisMaskr, &store.accounts[i].PublicViewKey, &store.accounts[i].PublicSpendKey, uint64(index))
		utxo.Keys = append(utxo.Keys, *key)
	}
	return utxo, nil
}

func (store storeImpl) CheckGhost(key crypto.Key) (bool, error) {
	return false, nil
}

func (store storeImpl) LockUTXOs(inputs []*Input, tx crypto.Hash, fork bool) error {
	return nil
}

func (store storeImpl) ReadDomains() []Domain {
	return nil
}

func (store storeImpl) ReadAllNodes(_ uint64, _ bool) []*Node {
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

func randomAccount() Address {
	seed := make([]byte, 64)
	rand.Read(seed)
	return NewAddressFromSeed(seed)
}

func TestTransactionV1(t *testing.T) {
	assert := assert.New(t)

	PM := "86a756657273696f6e01a54173736574c420a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdca6496e707574739285a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657800a747656e65736973c0a74465706f736974c0a44d696e74c085a448617368c4200000000000000000000000000000000000000000000000000000000000000000a5496e64657801a747656e65736973c0a74465706f736974c0a44d696e74c0a74f7574707574739285a45479706500a6416d6f756e74c70500e8d4a51000a44b657973c0a6536372697074c403fffe02a44d61736bc4204fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580585a45479706500a6416d6f756e74c70500e8d4a51000a44b65797393c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68a6536372697074c403fffe02a44d61736bc420c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ea54578747261c0aa5369676e617475726573c0"
	CM := "0000000028b52ffd63c118533ce8001d0800c40ca99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc921000024fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f2580593c42082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac420c06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fc420df02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb68c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53ec012fc75bb1394a785cc89cb470d24f85aedd083bc20068f76facb1fb825e9b432c19f5bf7a53b2c3bc0e7ce5e6fd9f8cbae17a5ec0e"

	accounts := make([]*Address, 0)
	for i := 0; i < 3; i++ {
		seed := make([]byte, 64)
		seed[i] = byte(i)
		a := NewAddressFromSeed(seed)
		accounts = append(accounts, &a)
	}

	seed := make([]byte, 64)
	rand.Read(seed)
	genesisHash := crypto.Hash{}
	script := Script{OperatorCmp, OperatorSum, 2}
	store := storeImpl{seed: seed, accounts: accounts}

	ver := NewTransaction(XINAssetId).AsLatestVersion()
	ver.Version = 1
	assert.Equal("d2cf4d6e85d76512b29f173073be167423705e207f090f8cfc3e2b61fc32b6e2", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 0)
	ver.resetCache()
	assert.Equal("b3afe7497740e05ba83e26977fbbfe7e1c2efc312d8d9aeb93bce43b9d8c6248", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 1)
	ver.resetCache()
	assert.Equal("e31ea7bd97a59169fbef1294b4dcc00dd33b6c4cd95367614415a5d6bdb1eee8", ver.PayloadHash().String())
	ver.Outputs = append(ver.Outputs, &Output{Type: OutputTypeScript, Amount: NewInteger(10000), Script: script, Mask: crypto.NewKeyFromSeed(bytes.Repeat([]byte{1}, 64))})
	ver.resetCache()
	assert.Equal("56fb588ab4319a54694fbbdc85f41b913401137da83ac6724e2c3adb076460f9", ver.PayloadHash().String())
	ver.AddScriptOutput(accounts, script, NewInteger(10000), bytes.Repeat([]byte{1}, 64))
	ver.resetCache()
	assert.Equal("d0a26a0a7f05941bc748b8f605f0b990511aafb865cf759364eb1d46156e6696", ver.PayloadHash().String())

	pm := ver.Marshal()
	assert.Equal(488, len(pm))
	assert.Equal(PM, hex.EncodeToString(pm))
	cm := ver.CompressMarshal()
	assert.Equal(277, len(cm))
	assert.Equal(CM, hex.EncodeToString(cm))
	ver, err := DecompressUnmarshalVersionedTransaction(cm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(488, len(pm))
	assert.Equal(PM, hex.EncodeToString(pm))
	ver, err = DecompressUnmarshalVersionedTransaction(pm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(488, len(pm))
	assert.Equal(PM, hex.EncodeToString(pm))
	cm, err = hex.DecodeString(CM)
	assert.Nil(err)
	ver, err = DecompressUnmarshalVersionedTransaction(cm)
	assert.Nil(err)
	pm = ver.Marshal()
	assert.Equal(488, len(pm))
	assert.Equal(PM, hex.EncodeToString(pm))

	for i := range ver.Inputs {
		err := ver.SignInputV1(store, i, accounts)
		if i == 0 {
			assert.NotNil(err)
			assert.Contains(err.Error(), "invalid key for the input")
		} else {
			assert.Nil(err)
		}
	}
	err = ver.Validate(store)
	assert.NotNil(err)
	assert.Contains(err.Error(), "invalid tx signature number")

	ver.SignaturesSliceV1 = nil
	for i := range ver.Inputs {
		err := ver.SignInputV1(store, i, accounts[0:i+1])
		assert.Nil(err)
	}
	err = ver.Validate(store)
	assert.Nil(err)

	outputs := ver.ViewGhostKey(&accounts[1].PrivateViewKey)
	assert.Len(outputs, 2)
	assert.Equal(outputs[1].Keys[1].String(), accounts[1].PublicSpendKey.String())
	outputs = ver.ViewGhostKey(&accounts[1].PrivateSpendKey)
	assert.Len(outputs, 2)
	assert.NotEqual(outputs[1].Keys[1].String(), accounts[1].PublicSpendKey.String())
	assert.NotEqual(outputs[1].Keys[1].String(), accounts[1].PublicViewKey.String())
}

func (ver *VersionedTransaction) resetCache() {
	ver.hash = crypto.Hash{}
	ver.pmbytes = nil
}
