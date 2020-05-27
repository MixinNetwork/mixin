package ed25519

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func BenchmarkMarshalKey(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		R := randomKey().Public().Key()
		var key crypto.Key
		s, _ := json.Marshal(R)
		if err := json.Unmarshal(s, &key); err != nil {
			b.Fatal(err)
		}
		if bytes.Compare(R[:], key[:]) != 0 {
			b.Fatal("unmarshal key not matched")
		}
	}
}

func TestKey(t *testing.T) {
	assert := assert.New(t)
	seed := make([]byte, 64)
	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 1)
	}
	key := NewPrivateKeyFromSeedOrPanic(seed)
	assert.Equal("c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a", key.Key().String())
	assert.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", key.Public().Key().String())

	j, err := key.Key().MarshalJSON()
	assert.Nil(err)
	assert.Equal("\"c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a\"", string(j))

	var k crypto.Key
	err = k.UnmarshalJSON(j)
	assert.Nil(err)
	priv, err := k.AsPrivateKey()
	assert.Nil(err)
	assert.Equal("c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a", priv.String())
	assert.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", priv.Public().Key().String())

	sig, err := key.Sign(seed)
	assert.Nil(err)
	assert.True(key.Public().Verify(seed, sig))
}

func randomKey() *Key {
	seed := make([]byte, 64)
	rand.Read(seed)
	return NewPrivateKeyFromSeedOrPanic(seed)
}
