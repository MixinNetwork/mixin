package crypto

import (
	"crypto/rand"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/assert"
	"go.dedis.ch/kyber/v3/suites"
)

func TestEdwards(t *testing.T) {
	assert := assert.New(t)

	seed := make([]byte, 64)
	rand.Read(seed)
	key := NewKeyFromSeed(seed)
	a, err := edwards25519.NewScalar().SetCanonicalBytes(key[:])
	assert.Nil(err)
	seed = make([]byte, 64)
	rand.Read(seed)
	key = NewKeyFromSeed(seed)
	b, err := edwards25519.NewScalar().SetCanonicalBytes(key[:])
	assert.Nil(err)

	p1 := edwards25519.NewIdentityPoint().ScalarBaseMult(a)
	p2 := edwards25519.NewIdentityPoint().ScalarMult(a, edwards25519.NewGeneratorPoint())
	assert.Equal(p1.Bytes(), p2.Bytes())

	s := suites.MustFind("ed25519")
	sa := s.Scalar().SetBytes(a.Bytes())
	sb := s.Scalar().SetBytes(b.Bytes())
	ss := s.Scalar().Add(sa, sb)
	tmp1 := edwards25519.NewScalar().Add(a, b)
	st := s.Scalar().SetBytes(tmp1.Bytes())
	assert.Equal(ss, st)
}
