package bulletproofs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"filippo.io/edwards25519"
	"filippo.io/edwards25519/field"
)

const (
	generatorDomain  = "bulletproof_plus"
	transcriptDomain = "bulletproof_plus_transcript"
)

var valueGeneratorEncoding = EncodedPoint{
	0x8b, 0x65, 0x59, 0x70, 0x15, 0x37, 0x99, 0xaf,
	0x2a, 0xea, 0xdc, 0x9f, 0xf1, 0xad, 0xd0, 0xea,
	0x6c, 0x72, 0x51, 0xd5, 0x41, 0x54, 0xcf, 0xa9,
	0x2c, 0x17, 0x3a, 0x0d, 0xd3, 0x9c, 0x1f, 0x94,
}

type generatorCache struct {
	value             *edwards25519.Point
	g                 []*edwards25519.Point
	h                 []*edwards25519.Point
	initialTranscript EncodedPoint
}

var loadGenerators = sync.OnceValue(func() *generatorCache {
	value, err := decodeCanonicalPoint(valueGeneratorEncoding)
	if err != nil {
		panic(err)
	}
	cache := &generatorCache{
		value: value,
		g:     make([]*edwards25519.Point, MaxCommitments*RangeBits),
		h:     make([]*edwards25519.Point, MaxCommitments*RangeBits),
	}

	base := make([]byte, 0, 32+len(generatorDomain)+binary.MaxVarintLen64)
	base = append(base, value.Bytes()...)
	base = append(base, generatorDomain...)
	for i := 0; i < len(cache.g); i++ {
		evenInput := binary.AppendUvarint(append([]byte(nil), base...), uint64(2*i))
		evenHash := keccak256(evenInput)
		cache.h[i] = biasedHashToPoint(evenHash[:])

		oddInput := binary.AppendUvarint(append([]byte(nil), base...), uint64(2*i+1))
		oddHash := keccak256(oddInput)
		cache.g[i] = biasedHashToPoint(oddHash[:])
	}

	initialHash := keccak256([]byte(transcriptDomain))
	copy(cache.initialTranscript[:], biasedHashToPoint(initialHash[:]).Bytes())
	return cache
})

func decodeCanonicalPoint(encoded EncodedPoint) (*edwards25519.Point, error) {
	p, err := edwards25519.NewIdentityPoint().SetBytes(encoded[:])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPoint, err)
	}
	if !bytes.Equal(encoded[:], p.Bytes()) {
		return nil, ErrInvalidPoint
	}
	return p, nil
}

func encodePoint(p *edwards25519.Point) (out EncodedPoint) {
	copy(out[:], p.Bytes())
	return out
}

func decodeCommitment(encoded Commitment) (*edwards25519.Point, error) {
	p, err := decodeCanonicalPoint(EncodedPoint(encoded))
	if err != nil {
		return nil, err
	}
	// A valid commitment is in the prime-order subgroup. Identity is allowed
	// for exact Monero compatibility (value=0 and blind=0).
	cleared := edwards25519.NewIdentityPoint().MultByCofactor(p)
	restored := edwards25519.NewIdentityPoint().ScalarMult(scalarInvEight, cleared)
	if restored.Equal(p) != 1 {
		return nil, ErrInvalidPoint
	}
	return p, nil
}

func scalePoint(p *edwards25519.Point, scalar *edwards25519.Scalar) *edwards25519.Point {
	return edwards25519.NewIdentityPoint().ScalarMult(scalar, p)
}

func pointFromUniformBytes(encoded [32]byte) *edwards25519.Point {
	var adjusted [32]byte
	copy(adjusted[:], encoded[:])
	msb := adjusted[31] >> 7
	adjusted[31] &= 0x7f

	r, err := new(field.Element).SetBytes(adjusted[:])
	if err != nil {
		panic(err)
	}
	if msb != 0 {
		var nineteenBytes [32]byte
		nineteenBytes[0] = 19
		nineteen, _ := new(field.Element).SetBytes(nineteenBytes[:])
		r.Add(r, nineteen)
	}

	one := new(field.Element).One()
	minusOne := new(field.Element).Negate(one)
	montgomeryA := fieldElementFromUint64(486662)
	minusA := new(field.Element).Negate(montgomeryA)

	var scratch0, scratch1, other field.Element
	rSquared := scratch0.Square(r)
	twoRSquared := rSquared.Add(rSquared, rSquared)
	onePlus := twoRSquared.Add(one, twoRSquared)
	onePlusInv := onePlus.Invert(onePlus)
	upsilon := onePlusInv.Multiply(minusA, onePlusInv)
	otherCandidate := other.Subtract(scratch1.Negate(upsilon), montgomeryA)

	var plusA, upsilonSquared, curveValue, root field.Element
	plusA.Add(upsilon, montgomeryA)
	upsilonSquared.Square(upsilon)
	curveValue.Multiply(&plusA, &upsilonSquared)
	curveValue.Add(&curveValue, upsilon)
	_, epsilon := root.SqrtRatio(&curveValue, one)
	u := r.Select(upsilon, otherCandidate, epsilon)
	if u.Equal(minusOne) == 1 {
		panic("bulletproofs+: invalid Elligator coordinate")
	}

	var numerator, denominator field.Element
	y := u.Multiply(
		numerator.Subtract(u, one),
		denominator.Invert(denominator.Add(u, one)),
	)
	var pointEncoding [32]byte
	copy(pointEncoding[:], y.Bytes())
	pointEncoding[31] ^= byte(epsilon << 7)
	p, err := edwards25519.NewIdentityPoint().SetBytes(pointEncoding[:])
	if err != nil {
		panic(err)
	}
	return p
}

func fieldElementFromUint64(value uint64) *field.Element {
	var encoded [32]byte
	binary.LittleEndian.PutUint64(encoded[:8], value)
	e, err := new(field.Element).SetBytes(encoded[:])
	if err != nil {
		panic(err)
	}
	return e
}

func biasedHashToPoint(data []byte) *edwards25519.Point {
	p := pointFromUniformBytes(keccak256(data))
	return edwards25519.NewIdentityPoint().MultByCofactor(p)
}

// Commit computes Monero's Pedersen commitment C = blind*G + value*H.
func Commit(value uint64, blind Scalar) (Commitment, error) {
	blindScalar, err := decodeScalar(blind)
	if err != nil {
		return Commitment{}, err
	}
	amount := scalarFromUint64(value)
	generators := loadGenerators()
	p := multiScalar(true,
		[]*edwards25519.Scalar{blindScalar, amount},
		[]*edwards25519.Point{edwards25519.NewGeneratorPoint(), generators.value},
	)
	var out Commitment
	copy(out[:], p.Bytes())
	return out, nil
}
