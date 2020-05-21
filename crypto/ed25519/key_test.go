package ed25519

import (
	"crypto/rand"
	"encoding/json"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func BenchmarkMarshalKey(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var key crypto.Key
		s, _ := json.Marshal(randomKey().Public().Key())
		json.Unmarshal(s, &key)
	}
}

func TestKey(t *testing.T) {
	assert := assert.New(t)
	seed := make([]byte, 64)
	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 1)
	}
	key := NewPrivateKeyFromSeedPanic(seed)
	assert.Equal("c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a", key.Key().String())
	assert.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", key.Public().Key().String())

	j, err := key.Key().MarshalJSON()
	assert.Nil(err)
	assert.Equal("\"c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a\"", string(j))

	var k crypto.Key
	err = k.UnmarshalJSON(j)
	assert.Nil(err)
	assert.Equal("c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a", k.String())
	assert.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", k.AsPrivateKeyPanic().Public().Key().String())

	sig := key.Sign(seed)
	assert.True(key.Public().Verify(seed, sig))
}

func randomKey() *Key {
	seed := make([]byte, 64)
	rand.Read(seed)
	return NewPrivateKeyFromSeedPanic(seed)
}
