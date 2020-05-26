package ed25519

import (
	"math/rand"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func BenchmarkDeriveGhostPrivateKey(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := randomKey()
		a := randomKey()
		b := randomKey()
		R := r.Public()
		outputIndex := uint64(rand.Int())
		crypto.DeriveGhostPrivateKey(R, a, b, outputIndex)
	}
}

func BenchmarkDeriveGhostPublicKey(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := randomKey()
		A := randomKey().Public()
		B := randomKey().Public()
		outputIndex := uint64(rand.Int())
		crypto.DeriveGhostPublicKey(r, A, B, outputIndex)
	}
}

func BenchmarkViewGhostOutputKey(b *testing.B) {
	b.ResetTimer()

	r := randomKey()
	a := randomKey()
	R := r.Public()
	A := a.Public()
	B := randomKey().Public()
	outputIndex := uint64(rand.Int())
	P := crypto.DeriveGhostPublicKey(r, A, B, outputIndex)
	for i := 0; i < b.N; i++ {
		crypto.ViewGhostOutputKey(R, P, a, outputIndex)
	}
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

	sig, err := p.Sign(a[:])
	assert.Nil(err)
	assert.True(P.Verify(a[:], sig))

	sig, err = a.Sign(a[:])
	assert.Nil(err)
	assert.True(A.Verify(a[:], sig))
}
