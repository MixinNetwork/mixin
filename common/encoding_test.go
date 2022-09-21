package common

import (
	"encoding/hex"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestEncoding(t *testing.T) {
	assert := assert.New(t)

	raw := "777700022dc0ab2919c77daea5cfc0b37a2beea02142e8fdc4f60409fd40b256bb13ea290007eff98bbf1fd4632380b3f81bec40b54ceaa5d10e181b2c9e141da28b3d13c5460001000000000000a348712f7881be7a7bec9935d46578fd612a96e1cd0ac0f83520e2c0db0e98e2000100000000000030ad61194c5c3c19c0397d3ae98fb25ea4ead720fd49a86f2ab7b6db888ea61b00010000000000005981c0b5df48c066b4b3858ea949990c82cc27e030127d9eda70ff57fe9d9feb0001000000000000ad2fccec444b26794a13fb52f71348308de20f72a6dc4544195cef79f60910660001000000000000c817d2cac077b5ab21af4166f7451d9678a836db3f709b2903a971bf763b7f890001000000000000a4df50c83ed97db449ec856d660f4b0ef1f888bf2824800cf1bcf71418b32e580001000000000000000200a100060417bce6c80000000000000000000000000000000000000000000000000000000000000000000000000077772dc0ab2919c77daea5cfc0b37a2beea02142e8fdc4f60409fd40b256bb13ea29002465656139303061382d623332372d343838632d386438642d313432383730326665323430006b344b45397734746e65417472324257736d6877693645624231436257716779424248326f4367397677676e39346e5a5a4d6379694c7655347a596b6277703277754e4a595651556b77795a46664e3846726238345178556770673174656e574c61647834554552583270480000000000053be744043b00012f4d6a6fd5720be42930533d2efd2f5659f6179ea0e677edad599ef1dc6293b8ae2ebb91eac8ceb937f92c7dd5ffdb577b6506c6fbe0f2c7baf35d399dbc7bab0003fffe010000001581a154c4107e519774e8784192b35a93ef68fff6ee0000"

	val, _ := hex.DecodeString(raw)
	signed, err := NewDecoder(val).DecodeTransaction()
	assert.Nil(err)

	w := signed.Outputs[0].Withdrawal
	assert.NotNil(w)
	assert.Equal("2dc0ab2919c77daea5cfc0b37a2beea02142e8fdc4f60409fd40b256bb13ea29", w.Chain.String())
	assert.Equal("eea900a8-b327-488c-8d8d-1428702fe240", w.AssetKey)
	assert.Equal("4KE9w4tneAtr2BWsmhwi6EbB1CbWqgyBBH2oCg9vwgn94nZZMcyiLvU4zYkbwp2wuNJYVQUkwyZFfN8Frb84QxUgpg1tenWLadx4UERX2pH", w.Address)
	assert.Equal("", w.Tag)

	enc := hex.EncodeToString(NewEncoder().EncodeTransaction(signed))
	assert.Equal(raw, enc)

	enc = hex.EncodeToString(signed.AsVersioned().PayloadMarshal())
	assert.Equal(raw, enc)
}

func TestAggregatedSignatureEncoding(t *testing.T) {
	assert := assert.New(t)

	for _, tp := range []struct {
		Signers []int
		Hex     string
		Len     int
	}{
		{
			[]int{0, 1, 2, 10, 28, 1056, 65535},
			"ffffff0100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010007000000010002000a001c0420ffff",
			85,
		},
		{
			[]int{0, 1, 2, 10, 11, 12, 13, 14, 15, 16, 17, 28},
			"ffffff010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000407fc0310",
			75,
		},
		{
			[]int{0, 1, 2, 3, 4, 5, 10, 11, 12, 13, 14, 15, 16, 17, 18, 28, 29, 30, 31},
			"ffffff01000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000043ffc07f0",
			75,
		},
		{
			[]int{0, 1, 2, 3, 4, 5, 10, 11, 12, 13, 14, 15, 16, 17, 18, 28, 29, 30, 128},
			"ffffff01000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000113ffc077000000000000000000000000001",
			88,
		},
	} {
		js := &AggregatedSignature{Signers: tp.Signers}
		enc := NewEncoder()
		enc.EncodeAggregatedSignature(js)
		jsb := enc.buf.Bytes()
		assert.Len(jsb, tp.Len)
		assert.Equal(tp.Hex, hex.EncodeToString(jsb))

		dec := NewDecoder(jsb)
		jh, err := dec.ReadInt()
		assert.Nil(err)
		assert.Equal(MaximumEncodingInt, jh)
		prefix, err := dec.ReadInt()
		assert.Nil(err)
		assert.Equal(AggregatedSignaturePrefix, prefix)
		djs, err := dec.ReadAggregatedSignature()
		assert.Nil(err)
		assert.Equal(js.Signers, djs.Signers)
	}
}

func TestCommonDataEncoding(t *testing.T) {
	assert := assert.New(t)

	mint := &MintDistribution{
		MintData: MintData{
			Group:  MintGroupKernelNode,
			Batch:  123,
			Amount: NewIntegerFromString("3.14159"),
		},
		Transaction: crypto.Blake3Hash([]byte("mint-test")),
	}

	enc := mint.Marshal()
	assert.Equal("777700010001000000000000007b000412b9af98eea889c227076f8c62106b59a478e043c0030392f3be0f5d714ed27953cb2668", hex.EncodeToString(enc))
	enc = mint.CompressMarshal()
	assert.Equal("0000000028b52ffd0300c118533ca10100777700010001000000000000007b000412b9af98eea889c227076f8c62106b59a478e043c0030392f3be0f5d714ed27953cb2668", hex.EncodeToString(enc))
	res, err := DecompressUnmarshalMintDistribution(enc)
	assert.Nil(err)
	assert.Equal(MintGroupKernelNode, res.Group)
	assert.Equal(uint64(123), res.Batch)
	assert.Equal("3.14159000", res.Amount.String())
	assert.Equal("eea889c227076f8c62106b59a478e043c0030392f3be0f5d714ed27953cb2668", res.Transaction.String())
	assert.Equal(msgpackMarshalPanic(res), msgpackMarshalPanic(mint))
}
