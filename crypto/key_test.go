package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGhostKey(t *testing.T) {
	assert := assert.New(t)
	a := randomKey()
	A := a.Public()
	b := randomKey()
	B := b.Public()
	r := randomKey()
	R := r.Public()

	P := DeriveGhostPublicKey(&r, &A, &B)
	p := DeriveGhostPrivateKey(&R, &a, &b)
	assert.Equal(*P, p.Public())

	O := ViewGhostOutputKey(P, &a, &R)
	assert.Equal(*O, B)

	sig := p.Sign(a[:])
	assert.True(P.Verify(a[:], sig))

	sig = a.Sign(a[:])
	assert.True(A.Verify(a[:], sig))
}

func randomKey() Key {
	seed := make([]byte, 64)
	rand.Read(seed)
	return NewKeyFromSeed(seed)
}
