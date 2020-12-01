package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/MixinNetwork/mixin/crypto/edwards25519"
	"github.com/stretchr/testify/assert"
	"go.dedis.ch/kyber/v3/suites"
)

func TestEdwards(t *testing.T) {
	assert := assert.New(t)

	var base = edwards25519.ExtendedGroupElement{
		X: edwards25519.FieldElement{25485296, 5318399, 8791791, -8299916, -14349720, 6939349, -3324311, -7717049, 7287234, -6577708},
		Y: edwards25519.FieldElement{-758052, -1832720, 13046421, -4857925, 6576754, 14371947, -13139572, 6845540, -2198883, -4003719},
		Z: edwards25519.FieldElement{-947565, 6097708, -469190, 10704810, -8556274, -15589498, -16424464, -16608899, 14028613, -5004649},
		T: edwards25519.FieldElement{6966464, -2456167, 7033433, 6781840, 28785542, 12262365, -2659449, 13959020, -21013759, -5262166},
	}

	seed := make([]byte, 64)
	rand.Read(seed)
	key := NewKeyFromSeed(seed)
	a := [32]byte(key)
	seed = make([]byte, 64)
	rand.Read(seed)
	key = NewKeyFromSeed(seed)
	b := [32]byte(key)

	var point1 edwards25519.ExtendedGroupElement
	edwards25519.GeScalarMultBase(&point1, &a)
	var tmp1 [32]byte
	point1.ToBytes(&tmp1)
	var point2 edwards25519.ProjectiveGroupElement
	edwards25519.GeScalarMult(&point2, &a, &base)
	var tmp2 [32]byte
	point2.ToBytes(&tmp2)
	assert.Equal(tmp1, tmp2)

	s := suites.MustFind("ed25519")
	sa := s.Scalar().SetBytes(a[:])
	sb := s.Scalar().SetBytes(b[:])
	ss := s.Scalar().Add(sa, sb)
	edwards25519.ScAdd(&tmp1, &a, &b)
	st := s.Scalar().SetBytes(tmp1[:])
	assert.Equal(ss, st)
}
