package ed25519

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkSignature(b *testing.B) {
	b.ResetTimer()
	var raw = []byte("just a test")
	for i := 0; i < b.N; i++ {
		p := randomKey()
		pub := Key(p.Public().Key())
		sig, _ := p.Sign(raw)
		pub.Verify(raw, sig)
	}
}

func TestSignature(t *testing.T) {
	assert := assert.New(t)

	seed := make([]byte, 64)
	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 1)
	}
	key1 := NewPrivateKeyFromSeedOrPanic(seed)
	pub1 := key1.Public()

	sig1, err := key1.Sign(seed[:32])
	assert.Nil(err)
	assert.Equal("466c0938b2e3cdc38c357b6aff88a75aa1c2ee987432a7d4703618e1d926ef6f1726017cfc49fd31344e2f6f8cce3ed7c4c43524e1889e45d842ee7cad0bf700", sig1.String())
	assert.False(randomKey().Verify(seed[:32], sig1))
	assert.False(randomKey().Verify(seed[32:], sig1))
	assert.True(pub1.Verify(seed[:32], sig1))
	assert.False(pub1.Verify(seed[32:], sig1))
	sig2, err := key1.Sign(seed[32:])
	assert.Nil(err)
	assert.Equal("b5a110f7c15cff4599470ef6d9c85cd1236c833493de283e8bbcf8bb4e54683355a9cbd5cfdb00b84034accd77320403fe9485715e0a99aecf62b01ece9d4c0d", sig2.String())
	assert.False(randomKey().Verify(seed[:32], sig2))
	assert.False(randomKey().Verify(seed[32:], sig2))
	assert.False(pub1.Verify(seed[:32], sig2))
	assert.True(pub1.Verify(seed[32:], sig2))

	seed2 := make([]byte, 64)
	copy(seed2, sig1[:])
	key2 := NewPrivateKeyFromSeedOrPanic(seed2)

	sig3, _ := key2.Sign(seed[:32])
	assert.Equal("6876535147490eca84e7c6402de90bb33bc8908431e7d7397bee13c740397c3726b7e8ae4169ef3459f3ee07e94d2b5324ae27585ff6e64e133cbecf56864c0d", sig3.String())
	sig4, _ := key2.Sign(seed[32:])
	assert.Equal("6e8012337b7dc9b901b87a8bb777eb2dd0ab95f5db45f5ce195d26622397d1a5b72416f1c4fcac46cc611001bd3f2e0edad82abb776f2ad0321d4cddf161380c", sig4.String())

	j, err := sig4.MarshalJSON()
	assert.Nil(err)
	assert.Equal("\"6e8012337b7dc9b901b87a8bb777eb2dd0ab95f5db45f5ce195d26622397d1a5b72416f1c4fcac46cc611001bd3f2e0edad82abb776f2ad0321d4cddf161380c\"", string(j))
	err = sig3.UnmarshalJSON(j)
	assert.Nil(err)
	assert.Equal("6e8012337b7dc9b901b87a8bb777eb2dd0ab95f5db45f5ce195d26622397d1a5b72416f1c4fcac46cc611001bd3f2e0edad82abb776f2ad0321d4cddf161380c", sig3.String())
}
