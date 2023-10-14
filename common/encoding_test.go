package common

import (
	"encoding/hex"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestIntEncoding(t *testing.T) {
	require := require.New(t)

	enc := NewEncoder()
	enc.WriteUint16(12)
	enc.WriteUint32(64)
	enc.WriteUint64(66667777)
	enc.WriteInt(129)
	require.Equal("000c000000400000000003f945010081", hex.EncodeToString(enc.Bytes()))

	dec := NewDecoder(enc.Bytes())
	u16, err := dec.ReadUint16()
	require.Nil(err)
	require.Equal(uint16(12), u16)
	u32, err := dec.ReadUint32()
	require.Nil(err)
	require.Equal(uint32(64), u32)
	u64, err := dec.ReadUint64()
	require.Nil(err)
	require.Equal(uint64(66667777), u64)
	i, err := dec.ReadInt()
	require.Nil(err)
	require.Equal(129, i)
}

func TestEncoding(t *testing.T) {
	require := require.New(t)

	raw := "777700052dc0ab2919c77daea5cfc0b37a2beea02142e8fdc4f60409fd40b256bb13ea290007eff98bbf1fd4632380b3f81bec40b54ceaa5d10e181b2c9e141da28b3d13c5460001000000000000a348712f7881be7a7bec9935d46578fd612a96e1cd0ac0f83520e2c0db0e98e2000100000000000030ad61194c5c3c19c0397d3ae98fb25ea4ead720fd49a86f2ab7b6db888ea61b00010000000000005981c0b5df48c066b4b3858ea949990c82cc27e030127d9eda70ff57fe9d9feb0001000000000000ad2fccec444b26794a13fb52f71348308de20f72a6dc4544195cef79f60910660001000000000000c817d2cac077b5ab21af4166f7451d9678a836db3f709b2903a971bf763b7f890001000000000000a4df50c83ed97db449ec856d660f4b0ef1f888bf2824800cf1bcf71418b32e580001000000000000000200a100060417bce6c8000000000000000000000000000000000000000000000000000000000000000000000000007777006b344b45397734746e65417472324257736d6877693645624231436257716779424248326f4367397677676e39346e5a5a4d6379694c7655347a596b6277703277754e4a595651556b77795a46664e3846726238345178556770673174656e574c61647834554552583270480000000000053be744043b00012f4d6a6fd5720be42930533d2efd2f5659f6179ea0e677edad599ef1dc6293b8ae2ebb91eac8ceb937f92c7dd5ffdb577b6506c6fbe0f2c7baf35d399dbc7bab0003fffe01000000000000001581a154c4107e519774e8784192b35a93ef68fff6ee0000"

	val, _ := hex.DecodeString(raw)
	signed, err := NewDecoder(val).DecodeTransaction()
	require.Nil(err)

	w := signed.Outputs[0].Withdrawal
	require.NotNil(w)
	require.Equal("4KE9w4tneAtr2BWsmhwi6EbB1CbWqgyBBH2oCg9vwgn94nZZMcyiLvU4zYkbwp2wuNJYVQUkwyZFfN8Frb84QxUgpg1tenWLadx4UERX2pH", w.Address)
	require.Equal("", w.Tag)

	enc := hex.EncodeToString(NewEncoder().EncodeTransaction(signed))
	require.Equal(raw, enc)

	enc = hex.EncodeToString(signed.AsVersioned().PayloadMarshal())
	require.Equal(raw, enc)

	raw = "77770005a99c2e0e2b1da4d648755ef19bd95139acbbe6564cfb06dec7cd34931ca72cdc0001c19d51beba90c20ff538a32ab262ce6e32e59f03b5bfe6d8e6fe2b2544ba43b60000000000000000000100a40005e8d4a510000000000000000000000000000000000000000000000000000000000000000000000000000000000000000040cf0926f381bb17668ef4b4eab6243d4b437ae6d2372623b74f41a5597277495556515cbc346d8b639386c1e22239d032bb6f09f8b6f2ea5a3a19b41fe0bdd1de0000"
	val, _ = hex.DecodeString(raw)
	signed, err = NewDecoder(val).DecodeTransaction()
	require.Nil(err)
	require.Equal("cf0926f381bb17668ef4b4eab6243d4b437ae6d2372623b74f41a5597277495556515cbc346d8b639386c1e22239d032bb6f09f8b6f2ea5a3a19b41fe0bdd1de", hex.EncodeToString(signed.Extra))
}

func TestAggregatedSignatureEncoding(t *testing.T) {
	require := require.New(t)

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
		require.Len(jsb, tp.Len)
		require.Equal(tp.Hex, hex.EncodeToString(jsb))

		dec := NewDecoder(jsb)
		jh, err := dec.ReadInt()
		require.Nil(err)
		require.Equal(MaximumEncodingInt, jh)
		prefix, err := dec.ReadInt()
		require.Nil(err)
		require.Equal(AggregatedSignaturePrefix, prefix)
		djs, err := dec.ReadAggregatedSignature()
		require.Nil(err)
		require.Equal(js.Signers, djs.Signers)
	}
}

func TestCommonDataEncoding(t *testing.T) {
	require := require.New(t)

	mint := &MintDistribution{
		MintData: MintData{
			Group:  mintGroupUniversal,
			Batch:  123,
			Amount: NewIntegerFromString("3.14159"),
		},
		Transaction: crypto.Blake3Hash([]byte("mint-test")),
	}

	enc := mint.Marshal()
	require.Equal("777700010000000000000000007b000412b9af98eea889c227076f8c62106b59a478e043c0030392f3be0f5d714ed27953cb2668", hex.EncodeToString(enc))
	enc = mint.Marshal()
	require.Equal("777700010000000000000000007b000412b9af98eea889c227076f8c62106b59a478e043c0030392f3be0f5d714ed27953cb2668", hex.EncodeToString(enc))
	res, err := UnmarshalMintDistribution(enc)
	require.Nil(err)
	require.Equal(mintGroupUniversal, res.Group)
	require.Equal(uint64(123), res.Batch)
	require.Equal("3.14159000", res.Amount.String())
	require.Equal("eea889c227076f8c62106b59a478e043c0030392f3be0f5d714ed27953cb2668", res.Transaction.String())
}
