package common

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestTransactionReferences(t *testing.T) {
	require := require.New(t)

	PM := "77770005a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc00020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000200000005e8d4a5100000004fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f258050003fffe0d000000000005e8d4a51000001041cd5439a3a3caf43b5755facd2856b8eb8dd9c825ddbdc4c2fc283afd25d428d069468da7057e644259c5f82cea4f32b481844aff68409a2823e6a2e7d84ae59402e07b4e453035787231b6b9b5c53498573e22e7f0d1440741c95e4c51b96c81ef7ed772d8f864f4a0250478fbc3c2927b7dd5dc364d6ad49156eccdde902c139921b524d87fafa4e671e6f8d9a9b3bbb405573eef90df4ea9d966c1a81b2d99e4228582ee9001653cfb2d7eb61dfe14d243e0280db8ffe2741a89190f532fbbbbe72344c65127e697a246c5f70804342195b92835afa9d8edf7498ba083e407a579b53eb7ce1ee7e97f826e6b463e7ad160cb97c56b6166d125ffd8b6f021d3f4a6136aaddce4bdbfddae92f702c56ccb94edb2f6d93615887f0806900a65c0f230e2e2ae9358beb7e7299cf8a00bc2fd2038540f818db6e16dd4abf4dadce64dd745fe693b2ee41e4ff1b7fccff3f50819a7d41e76cb04fe1065059f3b2068a5f51863e976f65e7b2665045e3e8919b96cae80cbbbf9d33009094b5091dde31937cf61a9d7393c6d4b01f068725f233eb564bb00767138b1c83bd09cf148832f8e5303a3249cee3c707607eb8ea030c0b92777e3ed729fb2aee4c4298bd6dcd0d1c0eff1a06c68bf6459f35c8a047130b631b22bff252edeb03310cf7f2121f21afb2d299f7febc6a3eaa79e5e19bd3a5c299817b50262289e2bc382f173c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53e0003fffe0d00000000000000000000"
	CM := "0000000028b52ffd4300c118533ce4019d1400a42777770005a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc00020105e8d4a5100000004fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f258050003fffe0d1041cd5439a3a3caf43b5755facd2856b8eb8dd9c825ddbdc4c2fc283afd25d428d069468da7057e644259c5f82cea4f32b481844aff68409a2823e6a2e7d84ae59402e07b4e453035787231b6b9b5c53498573e22e7f0d1440741c95e4c51b96c81ef7ed772d8f864f4a0250478fbc3c2927b7dd5dc364d6ad49156eccdde902c139921b524d87fafa4e671e6f8d9a9b3bbb405573eef90df4ea9d966c1a81b2d99e4228582ee9001653cfb2d7eb61dfe14d243e0280db8ffe2741a89190f532fbbbbe72344c65127e697a246c5f70804342195b92835afa9d8edf7498ba083e407a579b53eb7ce1ee7e97f826e6b463e7ad160cb97c56b6166d125ffd8b6f021d3f4a6136aaddce4bdbfddae92f702c56ccb94edb2f6d93615887f0806900a65c0f230e2e2ae9358beb7e7299cf8a00bc2fd2038540f818db6e16dd4abf4dadce64dd745fe693b2ee41e4ff1b7fccff3f50819a7d41e76cb04fe1065059f3b2068a5f51863e976f65e7b2665045e3e8919b96cae80cbbbf9d33009094b5091dde31937cf61a9d7393c6d4b01f068725f233eb564bb00767138b1c83bd09cf148832f8e5303a3249cee3c707607eb8ea030c0b92777e3ed729fb2aee4c4298bd6dcd0d1c0eff1a06c68bf6459f35c8a047130b631b22bff252edeb03310cf7f2121f21afb2d299f7febc6a3eaa79e5e19bd3a5c299817b50262289e2bc382f173c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53e00000000000700216a889d55a52db4aa823095819b3bad3baf993d01"

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
	require.Equal("814d45237cf84feb5be8cb60a3f985a019169a01a6d05924b74aa3493da02dd6", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 0)
	ver.resetCache()
	require.Equal("826aedc5f8ba217c399bfc798f2d4b7a169b7c9714397b56e32e609b9f5e0a6d", ver.PayloadHash().String())
	ver.AddInput(genesisHash, 1)
	ver.resetCache()
	require.Equal("61f00c8f14383c0a174543f2ba775f10ca38c91cf9f6c0b28de9bb9579fc2c51", ver.PayloadHash().String())
	ver.Outputs = append(ver.Outputs, &Output{Type: OutputTypeScript, Amount: NewInteger(10000), Script: script, Mask: crypto.NewKeyFromSeed(bytes.Repeat([]byte{1}, 64))})
	ver.resetCache()
	require.Equal("63a78e9776d6b4a0fe11702b825522554a68a5dcae6d0ca3aa7d4e9e5f66b0db", ver.PayloadHash().String())
	ver.AddScriptOutput(accounts, script, NewInteger(10000), bytes.Repeat([]byte{1}, 64))
	ver.resetCache()
	require.Equal("cf2f58aedcf4e85e12b533dfd39c396a2c5dd544f305b154f85c8c4fdfbda5bd", ver.PayloadHash().String())

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
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.NotNil(err)
	require.Contains(err.Error(), "invalid tx signature number")

	ver.SignaturesMap = nil
	for i := range ver.Inputs {
		err := ver.SignInput(store, i, accounts[0:i+1])
		require.Nil(err)
		err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
		if i < len(ver.Inputs)-1 {
			require.NotNil(err)
		} else {
			require.Nil(err)
		}
	}
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
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
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
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
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
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
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
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
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
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
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.NotNil(err)
	require.Contains(err.Error(), "invalid signatures map 2")
	ver.SignaturesMap = nil
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
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
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
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
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.NotNil(err)

	aas = make([][]*Address, len(ver.Inputs))
	for i := range ver.Inputs {
		aas[i] = append([]*Address{}, accounts[0:i+1]...)
	}
	ver.AggregatedSignature = nil
	err = ver.AggregateSign(store, aas, seed)
	require.Nil(err)
	require.Len(ver.AggregatedSignature.Signers, 3)
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.Nil(err)

	pm = ver.Marshal()
	require.Len(pm, 810)
	ver, err = DecompressUnmarshalVersionedTransaction(pm)
	require.Nil(err)
	require.NotNil(ver.AggregatedSignature)
	require.Nil(ver.SignaturesMap)
	require.Equal(pm, ver.Marshal())
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.Nil(err)

	require.Len(ver.References, 0)
	require.Len(ver.PayloadMarshal(), 740)
	ver, _ = DecompressUnmarshalVersionedTransaction(pm)
	ver.References = []crypto.Hash{ver.Inputs[0].Hash}
	require.Len(ver.PayloadMarshal(), 772)
	require.Len(ver.AggregatedSignature.Signers, 3)
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.NotNil(err)
	ver.AggregatedSignature = nil
	err = ver.AggregateSign(store, aas, seed)
	require.Nil(err)
	require.Len(ver.AggregatedSignature.Signers, 3)
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.Nil(err)
	pm = ver.Marshal()
	require.Len(pm, 842)
	ver, _ = DecompressUnmarshalVersionedTransaction(pm)
	require.Len(ver.References, 1)
	require.Equal(ver.Inputs[0].Hash, ver.References[0])
}

type storeImpl struct {
	custodian *Address
	seed      []byte
	accounts  []*Address
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

func (store storeImpl) LockGhostKeys(_ []*crypto.Key, _ crypto.Hash, _ bool) error {
	return nil
}

func (store storeImpl) LockUTXOs(_ []*Input, _ crypto.Hash, _ bool) error {
	return nil
}

func (store storeImpl) ReadAllNodes(_ uint64, _ bool) []*Node {
	return nil
}

func (store storeImpl) ReadTransaction(_ crypto.Hash) (*VersionedTransaction, string, error) {
	return nil, "", nil
}

func (store storeImpl) CheckDepositInput(_ *DepositData, _ crypto.Hash) error {
	return nil
}

func (store storeImpl) LockDepositInput(_ *DepositData, _ crypto.Hash, _ bool) error {
	return nil
}

func (store storeImpl) ReadLastMintDistribution(_ uint64) (*MintDistribution, error) {
	return nil, nil
}

func (store storeImpl) LockMintInput(_ *MintData, _ crypto.Hash, _ bool) error {
	return nil
}

func (store storeImpl) ReadCustodian(_ uint64) (*CustodianUpdateRequest, error) {
	return &CustodianUpdateRequest{Custodian: store.custodian}, nil
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
