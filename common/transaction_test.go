package common

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestTransactionReferences(t *testing.T) {
	require := require.New(t)

	PM := "77770004a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc00020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000200000005e8d4a5100000004fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f258050003fffe0d000000000005e8d4a51000001082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fdf02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb6832218c3a5ac643ff812bf9968fa545ea3862e8c103762e0eef25c4969ddb1cf262e1678c55a525f1be99c3168fd0d9e5aa4058046a0dace30c0eacca6570f976bb5214f113d3c99bf80c7336f9ce4a15af88e782cb3b912162db7c94a93ef12ffed7db88dbb7f9eb9b4ffd36493551ab1aecabc6d1153c9e5ce62599cfe68a28470d974e6e1397a055175082a606916d10becc943e01c39c1f40cf784d016ab28bc8c3e483b06ea5abb6c7f1f55683b903071205ed0c8d0a7079b647fdd8f49784d74d969eded1ab4fea0c98515bad32fbb7587a13de9e64f7ffd0d7b7d3c358867d3ece1fd8e73df21402b0585a359503ae28d5e57aaa47918a70fc2fe2c73855a3baacb8acf8e87830f70b28737cd91d6b733681da009d0d7a69de93eff57cfa973a8156c81379bf470c83a1c64dbb05e3dd060d87575dcc3b0b40d75b06719ef8473ab7400748532e593bd84405390b50ca0ef514b7a75bc74d9632183a4de891a54b45813fd35c739402dc1321c43da131722dff4befd6cfcaaa73cfa8054623dd0c98361eb656e5d9dfd6ec5332fa323f973e1693645fb7d06843898b91c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53e0003fffe0d00000000000000000000"
	CM := "0000000028b52ffd4300c118533ce4019d1400a42777770004a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc00020105e8d4a5100000004fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f258050003fffe0d1082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fdf02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb6832218c3a5ac643ff812bf9968fa545ea3862e8c103762e0eef25c4969ddb1cf262e1678c55a525f1be99c3168fd0d9e5aa4058046a0dace30c0eacca6570f976bb5214f113d3c99bf80c7336f9ce4a15af88e782cb3b912162db7c94a93ef12ffed7db88dbb7f9eb9b4ffd36493551ab1aecabc6d1153c9e5ce62599cfe68a28470d974e6e1397a055175082a606916d10becc943e01c39c1f40cf784d016ab28bc8c3e483b06ea5abb6c7f1f55683b903071205ed0c8d0a7079b647fdd8f49784d74d969eded1ab4fea0c98515bad32fbb7587a13de9e64f7ffd0d7b7d3c358867d3ece1fd8e73df21402b0585a359503ae28d5e57aaa47918a70fc2fe2c73855a3baacb8acf8e87830f70b28737cd91d6b733681da009d0d7a69de93eff57cfa973a8156c81379bf470c83a1c64dbb05e3dd060d87575dcc3b0b40d75b06719ef8473ab7400748532e593bd84405390b50ca0ef514b7a75bc74d9632183a4de891a54b45813fd35c739402dc1321c43da131722dff4befd6cfcaaa73cfa8054623dd0c98361eb656e5d9dfd6ec5332fa323f973e1693645fb7d06843898b91c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53e00000000000700216a889d55a52db4aa823095819b3bad3baf993d01"

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

	ver := NewTransactionV5(XINAssetId).AsVersioned()
	require.Equal("d9c6168b677ec72e351f05f7acc4c34fe8d9d606fa8645241b7407918a649c68", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 0)
	ver.resetCache()
	require.Equal("8f5b8ac9de4733db496b5fa6305cada963a2da703ecc2c101ee579c001ac12cb", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 1)
	ver.resetCache()
	require.Equal("1f4e5c9fec2eba9a4af1dd6917eba0bb7e26201002d0a75df11b9d605e2fe6b4", ver.PayloadHash().String())
	ver.Outputs = append(ver.Outputs, &Output{Type: OutputTypeScript, Amount: NewInteger(10000), Script: script, Mask: crypto.NewKeyFromSeed(bytes.Repeat([]byte{1}, 64))})
	ver.resetCache()
	require.Equal("3c4c47783aae1a492dbcfec32c2f8a27714367b92a89e7c792144cca913204dc", ver.PayloadHash().String())
	ver.AddScriptOutput(accounts, script, NewInteger(10000), bytes.Repeat([]byte{1}, 64))
	ver.resetCache()
	require.Equal("4637bb62c48a595e50468a099aa221a2f82f7c8d5be1ae872fed93d8665c07d6", ver.PayloadHash().String())

	pm := ver.Marshal()
	require.Equal(740, len(pm))
	require.Equal(PM, hex.EncodeToString(pm))
	cm := ver.CompressMarshal()
	require.Equal(678, len(cm))
	require.Equal(CM, hex.EncodeToString(cm))
	ver, err := DecompressUnmarshalVersionedTransaction(cm)
	require.Nil(err)
	pm = ver.Marshal()
	require.Equal(740, len(pm))
	require.Equal(PM, hex.EncodeToString(pm))
	ver, err = DecompressUnmarshalVersionedTransaction(pm)
	require.Nil(err)
	pm = ver.Marshal()
	require.Equal(740, len(pm))
	require.Equal(PM, hex.EncodeToString(pm))
	cm, err = hex.DecodeString(CM)
	require.Nil(err)
	ver, err = DecompressUnmarshalVersionedTransaction(cm)
	require.Nil(err)
	pm = ver.Marshal()
	require.Equal(740, len(pm))
	require.Equal(PM, hex.EncodeToString(pm))

	for i := range ver.Inputs {
		err := ver.SignInput(store, i, accounts)
		require.NotNil(err)
		require.Contains(err.Error(), "invalid key for the input")
	}
	err = ver.Validate(store, false)
	require.NotNil(err)
	require.Contains(err.Error(), "invalid tx signature number")

	ver.SignaturesMap = nil
	for i := range ver.Inputs {
		err := ver.SignInput(store, i, accounts[0:i+1])
		require.Nil(err)
		err = ver.Validate(store, false)
		if i < len(ver.Inputs)-1 {
			require.NotNil(err)
		} else {
			require.Nil(err)
		}
	}
	err = ver.Validate(store, false)
	require.Nil(err)

	pm = ver.Marshal()
	require.Len(pm, 942)
	ver, err = DecompressUnmarshalVersionedTransaction(pm)
	require.Nil(err)
	require.Nil(ver.AggregatedSignature)
	require.NotNil(ver.SignaturesMap)
	require.Equal(pm, ver.Marshal())

	require.Len(ver.Inputs, 2)
	require.Len(ver.SignaturesMap, 2)
	require.Len(ver.SignaturesMap[0], 1)
	require.Len(ver.SignaturesMap[1], 2)
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
	err = ver.Validate(store, false)
	require.NotNil(err)
	require.Equal("batch verification failure 3 3", err.Error())
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
	err = ver.Validate(store, false)
	require.NotNil(err)
	require.Equal("invalid signature map index 2 2", err.Error())
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
	err = ver.Validate(store, false)
	require.NotNil(err)
	require.Equal("batch verification failure 4 4", err.Error())
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
	err = ver.Validate(store, false)
	require.NotNil(err)
	require.Equal("batch verification failure 3 3", err.Error())

	outputs := ver.ViewGhostKey(&accounts[1].PrivateViewKey)
	require.Len(outputs, 2)
	require.Equal(outputs[1].Keys[1].String(), accounts[1].PublicSpendKey.String())
	outputs = ver.ViewGhostKey(&accounts[1].PrivateSpendKey)
	require.Len(outputs, 2)
	require.NotEqual(outputs[1].Keys[1].String(), accounts[1].PublicSpendKey.String())
	require.NotEqual(outputs[1].Keys[1].String(), accounts[1].PublicViewKey.String())

	ver.AggregatedSignature = &AggregatedSignature{}
	err = ver.Validate(store, false)
	require.NotNil(err)
	require.Contains(err.Error(), "invalid signatures map 2")
	ver.SignaturesMap = nil
	err = ver.Validate(store, false)
	require.NotNil(err)
	require.Contains(err.Error(), "invalid signature keys 0 1")

	aas := make([][]*Address, len(ver.Inputs))
	for i := range ver.Inputs {
		aas[i] = append([]*Address{}, accounts[0:i]...)
	}
	ver.AggregatedSignature = nil
	err = ver.AggregateSign(store, aas, seed)
	require.Nil(err)
	require.Len(ver.AggregatedSignature.Signers, 1)
	err = ver.Validate(store, false)
	require.NotNil(err)

	aas = make([][]*Address, len(ver.Inputs))
	for i := range ver.Inputs {
		accs := append([]*Address{}, accounts[0:i+1]...)
		accs[len(accs)-1], accs[0] = accs[0], accs[len(accs)-1]
		aas[i] = accs
	}
	ver.AggregatedSignature = nil
	err = ver.AggregateSign(store, aas, seed)
	require.NotNil(err)
	require.Nil(ver.AggregatedSignature)
	require.NotNil(ver.Marshal())
	err = ver.Validate(store, false)
	require.NotNil(err)

	aas = make([][]*Address, len(ver.Inputs))
	for i := range ver.Inputs {
		aas[i] = append([]*Address{}, accounts[0:i+1]...)
	}
	ver.AggregatedSignature = nil
	err = ver.AggregateSign(store, aas, seed)
	require.Nil(err)
	require.Len(ver.AggregatedSignature.Signers, 3)
	err = ver.Validate(store, false)
	require.Nil(err)

	pm = ver.Marshal()
	require.Len(pm, 810)
	ver, err = DecompressUnmarshalVersionedTransaction(pm)
	require.Nil(err)
	require.NotNil(ver.AggregatedSignature)
	require.Nil(ver.SignaturesMap)
	require.Equal(pm, ver.Marshal())
	err = ver.Validate(store, false)
	require.Nil(err)

	require.Len(ver.References, 0)
	require.Len(ver.PayloadMarshal(), 740)
	ver, _ = DecompressUnmarshalVersionedTransaction(pm)
	ver.References = []crypto.Hash{ver.Inputs[0].Hash}
	require.Len(ver.PayloadMarshal(), 772)
	require.Len(ver.AggregatedSignature.Signers, 3)
	err = ver.Validate(store, false)
	require.NotNil(err)
	ver.AggregatedSignature = nil
	err = ver.AggregateSign(store, aas, seed)
	require.Nil(err)
	require.Len(ver.AggregatedSignature.Signers, 3)
	err = ver.Validate(store, false)
	require.Nil(err)
	pm = ver.Marshal()
	require.Len(pm, 842)
	ver, _ = DecompressUnmarshalVersionedTransaction(pm)
	require.Len(ver.References, 1)
	require.Equal(ver.Inputs[0].Hash, ver.References[0])
}

type storeImpl struct {
	seed     []byte
	accounts []*Address
	domains  []*Domain
}

func (store storeImpl) ReadUTXOKeys(hash crypto.Hash, index int) (*UTXOKeys, error) {
	utxo, err := store.ReadUTXOLock(hash, index)
	if err != nil {
		return nil, err
	}
	return &UTXOKeys{
		Mask: utxo.Mask,
		Keys: utxo.Keys,
	}, nil
}

func (store storeImpl) ReadUTXOLock(hash crypto.Hash, index int) (*UTXOWithLock, error) {
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
		utxo.Keys = append(utxo.Keys, key)
	}
	return utxo, nil
}

func (store storeImpl) LockGhostKeys(keys []*crypto.Key, tx crypto.Hash, fork bool) error {
	return nil
}

func (store storeImpl) LockUTXOs(inputs []*Input, tx crypto.Hash, fork bool) error {
	return nil
}

func (store storeImpl) ReadDomains() []*Domain {
	return store.domains
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

func (store storeImpl) ReadLastMintDistribution(batch uint64) (*MintDistribution, error) {
	return nil, nil
}

func (store storeImpl) LockMintInput(mint *MintData, tx crypto.Hash, fork bool) error {
	return nil
}

func (store storeImpl) ReadCustodian(ts uint64) (*CustodianUpdateRequest, error) {
	return nil, nil
}

func randomAccount() Address {
	seed := make([]byte, 64)
	rand.Read(seed)
	return NewAddressFromSeed(seed)
}

func (ver *VersionedTransaction) resetCache() {
	ver.hash = crypto.Hash{}
	ver.pmbytes = nil
}
