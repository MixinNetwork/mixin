package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKey(t *testing.T) {
	require := require.New(t)
	seed := make([]byte, 64)
	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 1)
	}
	key := NewKeyFromSeed(seed)
	require.Equal("c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a", key.String())
	require.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", key.Public().String())

	j, err := key.MarshalJSON()
	require.Nil(err)
	require.Equal("\"c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a\"", string(j))
	err = key.UnmarshalJSON(j)
	require.Nil(err)
	require.Equal("c91e0907d114fd83c1edc396490bb2dafa43c19815b0354e70dc80c317c3cb0a", key.String())
	require.Equal("36bb0e309e7e9a82f1527df2c6b0e48181589097fe90c1282c558207ea27ce66", key.Public().String())
}

func TestGhostKey(t *testing.T) {
	require := require.New(t)
	a := randomKey()
	A := a.Public()
	b := randomKey()
	B := b.Public()
	r := randomKey()
	R := r.Public()

	P := DeriveGhostPublicKey(&r, &A, &B, 0)
	p := DeriveGhostPrivateKey(&A, &b, &r, 0)
	require.NotEqual(*P, p.Public())
	p = DeriveGhostPrivateKey(&B, &r, &a, 0)
	require.NotEqual(*P, p.Public())
	p = DeriveGhostPrivateKey(&B, &a, &r, 0)
	require.NotEqual(*P, p.Public())
	p = DeriveGhostPrivateKey(&A, &r, &b, 0)
	require.Equal(*P, p.Public())
	p = DeriveGhostPrivateKey(&R, &a, &b, 0)
	require.Equal(*P, p.Public())

	O := ViewGhostOutputKey(P, &a, &R, 0)
	require.Equal(*O, B)

	sig := p.Sign(a[:])
	require.True(P.Verify(a[:], sig))

	sig = a.Sign(a[:])
	require.True(A.Verify(a[:], sig))
}

func randomKey() Key {
	seed := make([]byte, 64)
	rand.Read(seed)
	return NewKeyFromSeed(seed)
}
