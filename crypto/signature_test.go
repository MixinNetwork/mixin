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

	sig1 := key1.Sign(seed[:32])
	require.Equal("466c0938b2e3cdc38c357b6aff88a75aa1c2ee987432a7d4703618e1d926ef6f1726017cfc49fd31344e2f6f8cce3ed7c4c43524e1889e45d842ee7cad0bf700", sig1.String())

	seed2 := make([]byte, 64)
	copy(seed2, sig1[:])
	key2 := NewKeyFromSeed(seed2)
	pub2 := key2.Public()

	require.False(pub2.Verify(seed[:32], sig1))
	require.False(pub2.Verify(seed[32:], sig1))
	require.True(pub1.Verify(seed[:32], sig1))
	require.False(pub1.Verify(seed[32:], sig1))
	sig2 := key1.Sign(seed[32:])
	require.Equal("b5a110f7c15cff4599470ef6d9c85cd1236c833493de283e8bbcf8bb4e54683355a9cbd5cfdb00b84034accd77320403fe9485715e0a99aecf62b01ece9d4c0d", sig2.String())
	require.False(pub2.Verify(seed[:32], sig2))
	require.False(pub2.Verify(seed[32:], sig2))
	require.False(pub1.Verify(seed[:32], sig2))
	require.True(pub1.Verify(seed[32:], sig2))

	stdPub := ed25519.PublicKey(pub1[:])
	require.True(ed25519.Verify(stdPub, seed[:32], sig1[:]))
	require.False(ed25519.Verify(stdPub, seed[32:], sig1[:]))
	require.False(ed25519.Verify(stdPub, seed[:32], sig2[:]))
	require.True(ed25519.Verify(stdPub, seed[32:], sig2[:]))
	res := BatchVerify(seed[:32], []*Key{}, []*Signature{})
	require.False(res)
	res = BatchVerify(seed[:32], []*Key{&pub1}, []*Signature{&sig1})
	require.True(res)
	res = BatchVerify(seed[:32], []*Key{&pub1, &pub1}, []*Signature{&sig1, &sig1})
	require.True(res)
	res = BatchVerify(seed[:32], []*Key{&pub1, &pub2}, []*Signature{&sig1, &sig2})
	require.False(res)

	sig3 := key2.Sign(seed[:32])
	require.Equal("6876535147490eca84e7c6402de90bb33bc8908431e7d7397bee13c740397c3726b7e8ae4169ef3459f3ee07e94d2b5324ae27585ff6e64e133cbecf56864c0d", sig3.String())
	sig4 := key2.Sign(seed[32:])
	require.Equal("6e8012337b7dc9b901b87a8bb777eb2dd0ab95f5db45f5ce195d26622397d1a5b72416f1c4fcac46cc611001bd3f2e0edad82abb776f2ad0321d4cddf161380c", sig4.String())
	res = BatchVerify(seed[:32], []*Key{&pub1, &pub2}, []*Signature{&sig1, &sig3})
	require.True(res)
	res = BatchVerify(seed[32:], []*Key{&pub1, &pub2}, []*Signature{&sig2, &sig4})
	require.True(res)

	j, err := sig4.MarshalJSON()
	require.Nil(err)
	require.Equal("\"6e8012337b7dc9b901b87a8bb777eb2dd0ab95f5db45f5ce195d26622397d1a5b72416f1c4fcac46cc611001bd3f2e0edad82abb776f2ad0321d4cddf161380c\"", string(j))
	err = sig3.UnmarshalJSON(j)
	require.Nil(err)
	require.Equal("6e8012337b7dc9b901b87a8bb777eb2dd0ab95f5db45f5ce195d26622397d1a5b72416f1c4fcac46cc611001bd3f2e0edad82abb776f2ad0321d4cddf161380c", sig3.String())
}
