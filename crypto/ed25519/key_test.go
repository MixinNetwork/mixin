package ed25519

import (
	"crypto/rand"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

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

func TestGhostKey(t *testing.T) {
	assert := assert.New(t)
	a := randomKey()
	A := a.Public()
	b := randomKey()
	B := b.Public()
	r := randomKey()
	R := r.Public()

	P := crypto.DeriveGhostPublicKey(r, A, B, 0)
	p := crypto.DeriveGhostPrivateKey(A, b, r, 0)
	assert.NotEqual(P.Key().String(), p.Public().Key().String())
	p = crypto.DeriveGhostPrivateKey(B, r, a, 0)
	assert.NotEqual(P.Key().String(), p.Public().Key().String())
	p = crypto.DeriveGhostPrivateKey(B, a, r, 0)
	assert.NotEqual(P.Key().String(), p.Public().Key().String())
	p = crypto.DeriveGhostPrivateKey(A, r, b, 0)
	assert.Equal(P.Key().String(), p.Public().Key().String())
	p = crypto.DeriveGhostPrivateKey(R, a, b, 0)
	assert.Equal(P.Key().String(), p.Public().Key().String())

	O := crypto.ViewGhostOutputKey(R, P, a, 0)
	assert.Equal(O.Key().String(), B.Key().String())

	sig := p.Sign(a[:])
	assert.True(P.Verify(a[:], sig))

	sig = a.Sign(a[:])
	assert.True(A.Verify(a[:], sig))
}

func randomKey() *Key {
	seed := make([]byte, 64)
	rand.Read(seed)
	return NewPrivateKeyFromSeedPanic(seed)
}
