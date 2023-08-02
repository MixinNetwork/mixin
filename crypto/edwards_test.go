package crypto

import (
	"crypto/rand"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v3/suites"
)

func TestEdwards(t *testing.T) {
	require := require.New(t)

	seed := make([]byte, 64)
	rand.Read(seed)
	key := NewKeyFromSeed(seed)
	a, err := edwards25519.NewScalar().SetCanonicalBytes(key[:])
	require.Nil(err)
	seed = make([]byte, 64)
	rand.Read(seed)
	key = NewKeyFromSeed(seed)
	b, err := edwards25519.NewScalar().SetCanonicalBytes(key[:])
	require.Nil(err)

	p1 := edwards25519.NewIdentityPoint().ScalarBaseMult(a)
	p2 := edwards25519.NewIdentityPoint().ScalarMult(a, edwards25519.NewGeneratorPoint())
	require.Equal(p1.Bytes(), p2.Bytes())

	s := suites.MustFind("ed25519")
	sa := s.Scalar().SetBytes(a.Bytes())
	sb := s.Scalar().SetBytes(b.Bytes())
	ss := s.Scalar().Add(sa, sb)
	tmp1 := edwards25519.NewScalar().Add(a, b)
	st := s.Scalar().SetBytes(tmp1.Bytes())
	require.Equal(ss, st)
}
