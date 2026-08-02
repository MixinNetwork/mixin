package bulletproofs

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/sha3"
)

var scalarLimit = [32]byte{
	0xe3, 0x6a, 0x67, 0x72, 0x8b, 0xce, 0x13, 0x29,
	0x8f, 0x30, 0x82, 0x8c, 0x0b, 0xa4, 0x10, 0x39,
	0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xf0,
}

var (
	scalarZero     = edwards25519.NewScalar()
	scalarOne      = mustScalarFromUint64(1)
	scalarEight    = mustScalarFromUint64(8)
	scalarInvEight = new(edwards25519.Scalar).Invert(scalarEight)
)

func mustScalarFromUint64(value uint64) *edwards25519.Scalar {
	var encoded [32]byte
	binary.LittleEndian.PutUint64(encoded[:8], value)
	s, err := edwards25519.NewScalar().SetCanonicalBytes(encoded[:])
	if err != nil {
		panic(err)
	}
	return s
}

func scalarFromUint64(value uint64) *edwards25519.Scalar {
	return new(edwards25519.Scalar).Set(mustScalarFromUint64(value))
}

func decodeScalar(encoded Scalar) (*edwards25519.Scalar, error) {
	s, err := edwards25519.NewScalar().SetCanonicalBytes(encoded[:])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidScalar, err)
	}
	return s, nil
}

func encodeScalar(s *edwards25519.Scalar) (out Scalar) {
	copy(out[:], s.Bytes())
	return out
}

// RandomScalar returns a non-zero uniformly distributed canonical scalar. The
// reader defaults to crypto/rand.Reader when nil.
func RandomScalar(random io.Reader) (Scalar, error) {
	s, err := randomScalar(random)
	if err != nil {
		return Scalar{}, err
	}
	return encodeScalar(s), nil
}

// randomScalar matches Monero's random32_unbiased rejection sampling. Keeping
// this behavior also makes deterministic prover comparisons possible.
func randomScalar(random io.Reader) (*edwards25519.Scalar, error) {
	if random == nil {
		random = cryptorand.Reader
	}
	var encoded [32]byte
	for {
		if _, err := io.ReadFull(random, encoded[:]); err != nil {
			return nil, fmt.Errorf("bulletproofs+: read randomness: %w", err)
		}
		if !scalarBelowLimit(encoded) {
			continue
		}
		s := reduceScalar32(encoded)
		if s.Equal(scalarZero) == 0 {
			return s, nil
		}
	}
}

func scalarBelowLimit(encoded [32]byte) bool {
	for i := len(encoded) - 1; i >= 0; i-- {
		if encoded[i] < scalarLimit[i] {
			return true
		}
		if encoded[i] > scalarLimit[i] {
			return false
		}
	}
	return false
}

func reduceScalar32(encoded [32]byte) *edwards25519.Scalar {
	var wide [64]byte
	copy(wide[:32], encoded[:])
	s, err := edwards25519.NewScalar().SetUniformBytes(wide[:])
	if err != nil {
		panic(err)
	}
	return s
}

func keccak256(parts ...[]byte) (out [32]byte) {
	h := sha3.NewLegacyKeccak256()
	for _, part := range parts {
		_, _ = h.Write(part)
	}
	copy(out[:], h.Sum(nil))
	return out
}

func hashToScalar(parts ...[]byte) *edwards25519.Scalar {
	return reduceScalar32(keccak256(parts...))
}

func scalarIsZero(s *edwards25519.Scalar) bool {
	return s.Equal(scalarZero) == 1
}

func scalarPowers(x *edwards25519.Scalar, count int) scalarVector {
	if count == 0 {
		return nil
	}
	result := make(scalarVector, count)
	result[0].Set(x)
	for i := 1; i < count; i++ {
		result[i].Multiply(&result[i-1], x)
	}
	return result
}
