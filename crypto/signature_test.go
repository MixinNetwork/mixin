package crypto

import (
	"crypto/ed25519"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignature(t *testing.T) {
	require := require.New(t)

	for i := 0; i < 5; i++ {
		testSignature(require)
	}
}

func testSignature(require *require.Assertions) {
	seed := make([]byte, 64)
	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 1)
	}
	key1 := NewKeyFromSeed(seed)
	pub1 := key1.Public()

	msg1 := Blake3Hash(seed[:32])
	msg2 := Blake3Hash(seed[32:])
	sig1 := key1.Sign(msg1)
	require.Equal("ca22e4ad608bad7638072e420ff5dd45eeb82c8e732e67580413066edf4cb8c0e2e6642f9e08b698a553a51e5719610b55d5afd1a9b9f3c69c6c60434dd55707", sig1.String())

	seed2 := make([]byte, 64)
	copy(seed2, sig1[:])
	key2 := NewKeyFromSeed(seed2)
	pub2 := key2.Public()

	require.False(pub2.Verify(msg1, sig1))
	require.False(pub2.Verify(msg2, sig1))
	require.True(pub1.Verify(msg1, sig1))
	require.False(pub1.Verify(msg2, sig1))
	sig2 := key1.Sign(msg2)
	require.Equal("ae7bde216edf1f6b00d1d87a584b8ec433913ee7028d075b3a11aec13aea86ed472bb45976100d9e679180d1a7afce352304347fefe6bd64b407097c6f887208", sig2.String())
	require.False(pub2.Verify(msg1, sig2))
	require.False(pub2.Verify(msg2, sig2))
	require.False(pub1.Verify(msg1, sig2))
	require.True(pub1.Verify(msg2, sig2))

	stdPub := ed25519.PublicKey(pub1[:])
	require.True(ed25519.Verify(stdPub, msg1[:], sig1[:]))
	require.False(ed25519.Verify(stdPub, msg2[:], sig1[:]))
	require.False(ed25519.Verify(stdPub, msg1[:], sig2[:]))
	require.True(ed25519.Verify(stdPub, msg2[:], sig2[:]))
	res := BatchVerify(msg1, []*Key{}, []*Signature{})
	require.False(res)
	res = BatchVerify(msg1, []*Key{&pub1}, []*Signature{&sig1})
	require.True(res)
	res = BatchVerify(msg1, []*Key{&pub1, &pub1}, []*Signature{&sig1, &sig1})
	require.True(res)
	res = BatchVerify(msg1, []*Key{&pub1, &pub2}, []*Signature{&sig1, &sig2})
	require.False(res)

	sig3 := key2.Sign(msg1)
	require.Equal("56e783313e7969ad3feab28f3f55917c44e7a5712818f86c4f92ac86a848696c778c10c1940f13e8b42e562bb7b8f789a87ca3bca4269f6c7107fc6ddf0e650c", sig3.String())
	sig4 := key2.Sign(msg2)
	require.Equal("7718b4dc9daeb8272132dbf7d52ca701d210a3bc044f0fc70494c035e6fd3df80ede5427c7e19832be6fa87b7cf0c24e72af1911a254ce708489d1bcd324ee06", sig4.String())
	res = BatchVerify(msg1, []*Key{&pub1, &pub2}, []*Signature{&sig1, &sig3})
	require.True(res)
	res = BatchVerify(msg2, []*Key{&pub1, &pub2}, []*Signature{&sig2, &sig4})
	require.True(res)

	j, err := sig4.MarshalJSON()
	require.Nil(err)
	require.Equal("\"7718b4dc9daeb8272132dbf7d52ca701d210a3bc044f0fc70494c035e6fd3df80ede5427c7e19832be6fa87b7cf0c24e72af1911a254ce708489d1bcd324ee06\"", string(j))
	err = sig3.UnmarshalJSON(j)
	require.Nil(err)
	require.Equal("7718b4dc9daeb8272132dbf7d52ca701d210a3bc044f0fc70494c035e6fd3df80ede5427c7e19832be6fa87b7cf0c24e72af1911a254ce708489d1bcd324ee06", sig3.String())
}
