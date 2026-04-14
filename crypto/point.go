package crypto

import (
	"bytes"
	"fmt"

	"filippo.io/edwards25519"
)

var invEightScalar = func() *edwards25519.Scalar {
	var eightBytes [32]byte
	eightBytes[0] = 8
	eight, err := edwards25519.NewScalar().SetCanonicalBytes(eightBytes[:])
	if err != nil {
		panic(err)
	}
	return edwards25519.NewScalar().Invert(eight)
}()

func decodePoint(src []byte) (*edwards25519.Point, error) {
	p, err := edwards25519.NewIdentityPoint().SetBytes(src)
	if err != nil {
		return nil, err
	}
	if !isPrimeOrderPoint(p) {
		return nil, fmt.Errorf("invalid point subgroup")
	}
	if !bytes.Equal(src, p.Bytes()) {
		return nil, fmt.Errorf("invalid point encoding")
	}
	return p, nil
}

func isPrimeOrderPoint(p *edwards25519.Point) bool {
	if p.Equal(edwards25519.NewIdentityPoint()) == 1 {
		return false
	}
	cleared := edwards25519.NewIdentityPoint().MultByCofactor(p)
	restored := edwards25519.NewIdentityPoint().ScalarMult(invEightScalar, cleared)
	return restored.Equal(p) == 1
}
