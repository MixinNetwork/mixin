package crypto

import (
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

func TestEdwards(t *testing.T) {
	require := require.New(t)

	seed := make([]byte, 64)
	ReadRand(seed)
	key := NewKeyFromSeed(seed)
	a, err := edwards25519.NewScalar().SetCanonicalBytes(key[:])
	require.Nil(err)
	seed = make([]byte, 64)
	ReadRand(seed)
	key = NewKeyFromSeed(seed)
	b, err := edwards25519.NewScalar().SetCanonicalBytes(key[:])
	require.Nil(err)

	p1 := edwards25519.NewIdentityPoint().ScalarBaseMult(a)
	p2 := edwards25519.NewIdentityPoint().ScalarMult(a, edwards25519.NewGeneratorPoint())
	require.Equal(p1.Bytes(), p2.Bytes())

	p2 = edwards25519.NewIdentityPoint().ScalarBaseMult(b)
	s := edwards25519.NewScalar().Add(a, b)
	copy(key[:], s.Bytes())
	S := key.Public()
	P := edwards25519.NewIdentityPoint().Add(p1, p2)
	require.Equal(S[:], P.Bytes())
}
