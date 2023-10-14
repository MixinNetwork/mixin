package common

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestTransactionExtraLimit(t *testing.T) {
	require := require.New(t)
	accounts := make([]*Address, 0)
	for i := 0; i < 16; i++ {
		seed := make([]byte, 64)
		seed[i] = byte(i)
		a := NewAddressFromSeed(seed)
		accounts = append(accounts, &a)
	}

	seed := make([]byte, 64)
	crypto.ReadRand(seed)
	store := storeImpl{seed: seed, accounts: accounts}

	CM := "77770005a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc00020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001000000000000000200000005e8d4a5100000004fe2a684e0e6c5e370ca0d89f5e2cb0da1e2ecd4028fa2d395fbca4e33f258050003fffe0d000000000005e8d4a51000001082240709ab6152f66d2887c78f4f13d2a9fcea5aab7ac48e8099bcb8e107173ac06fa8fd6bc52ada96cef6ea8da9ed1cdfb9bafbb7b4e345c827f7ae64c2353fdf02b12f33cc261928ede939cb146533730a0fc4e2cabbe973e4cf90bdadfb6832218c3a5ac643ff812bf9968fa545ea3862e8c103762e0eef25c4969ddb1cf262e1678c55a525f1be99c3168fd0d9e5aa4058046a0dace30c0eacca6570f976bb5214f113d3c99bf80c7336f9ce4a15af88e782cb3b912162db7c94a93ef12ffed7db88dbb7f9eb9b4ffd36493551ab1aecabc6d1153c9e5ce62599cfe68a28470d974e6e1397a055175082a606916d10becc943e01c39c1f40cf784d016ab28bc8c3e483b06ea5abb6c7f1f55683b903071205ed0c8d0a7079b647fdd8f49784d74d969eded1ab4fea0c98515bad32fbb7587a13de9e64f7ffd0d7b7d3c358867d3ece1fd8e73df21402b0585a359503ae28d5e57aaa47918a70fc2fe2c73855a3baacb8acf8e87830f70b28737cd91d6b733681da009d0d7a69de93eff57cfa973a8156c81379bf470c83a1c64dbb05e3dd060d87575dcc3b0b40d75b06719ef8473ab7400748532e593bd84405390b50ca0ef514b7a75bc74d9632183a4de891a54b45813fd35c739402dc1321c43da131722dff4befd6cfcaaa73cfa8054623dd0c98361eb656e5d9dfd6ec5332fa323f973e1693645fb7d06843898b91c6473159e19ed185b373e935081774e0c133b9416abdff319667187a71dff53e0003fffe0d00000000000000000000"
	cm, _ := hex.DecodeString(CM)
	ver, err := UnmarshalVersionedTransaction(cm)
	require.Nil(err)
	require.Equal(TxVersionHashSignature, int(ver.Version))
	require.Equal(CM, hex.EncodeToString(ver.Marshal()))
	require.Equal(ExtraSizeGeneralLimit, ver.getExtraLimit())

	ver.Outputs[1].Amount = NewIntegerFromString("0.001")
	require.Equal(ExtraSizeGeneralLimit, ver.getExtraLimit())
	ver.Outputs[1].Script = NewThresholdScript(64)
	require.Equal(ExtraSizeGeneralLimit, ver.getExtraLimit())

	ver, _ = UnmarshalVersionedTransaction(cm)
	ver.Outputs[0].Amount = NewIntegerFromString("0.001")
	require.Equal(ExtraSizeGeneralLimit, ver.getExtraLimit())
	ver.Outputs[0].Script = NewThresholdScript(64)
	require.Equal(ExtraSizeGeneralLimit, ver.getExtraLimit())
	ver.Outputs[0].Script = NewThresholdScript(63)
	ver.Outputs[0].Keys = []*crypto.Key{&accounts[2].PublicSpendKey}
	require.Equal(ExtraSizeGeneralLimit, ver.getExtraLimit())

	ver.Extra = bytes.Repeat([]byte{0}, 257)
	aas := make([][]*Address, len(ver.Inputs))
	for i := range ver.Inputs {
		aas[i] = append([]*Address{}, accounts[0:i+1]...)
	}
	ver.pmbytes = nil
	ver.AggregatedSignature = nil
	err = ver.AggregateSign(store, aas, seed)
	require.Nil(err)
	require.Len(ver.AggregatedSignature.Signers, 3)
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.NotNil(err)
	require.Contains(err.Error(), "invalid extra size 257")

	ver.Outputs[0].Script = NewThresholdScript(64)
	require.Equal(ExtraSizeStorageStep, ver.getExtraLimit())
	ver.Outputs[0].Amount = NewIntegerFromString("0.0015")
	require.Equal(ExtraSizeStorageStep, ver.getExtraLimit())
	ver.Outputs[0].Amount = NewIntegerFromString("0.0155")
	require.Equal(ExtraSizeStorageStep*15, ver.getExtraLimit())
	ver.Outputs[0].Amount = NewIntegerFromString("4.0959")
	require.Equal(ExtraSizeStorageStep*4095, ver.getExtraLimit())
	ver.Outputs[0].Amount = NewIntegerFromString("4.0969")
	require.Equal(ExtraSizeStorageStep*4096, ver.getExtraLimit())
	ver.Outputs[0].Amount = NewIntegerFromString("40.969")
	require.Equal(ExtraSizeStorageStep*4096, ver.getExtraLimit())

	ver.Outputs[1].Amount = NewIntegerFromString("20000").Sub(NewIntegerFromString("40.969"))
	ver.Extra = bytes.Repeat([]byte{0}, ExtraSizeStorageStep*4096-772)
	ver.pmbytes = nil
	ver.AggregatedSignature = nil
	err = ver.AggregateSign(store, aas, seed)
	require.Nil(err)
	require.Len(ver.AggregatedSignature.Signers, 3)
	err = ver.Validate(store, uint64(time.Now().UnixNano()), false)
	require.Nil(err)
}
