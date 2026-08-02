package bulletproofs

import (
	"errors"
	"fmt"
)

const (
	// RangeBits is the number of bits proven for every committed value.
	RangeBits = 64
	// MaxCommitments is Monero's maximum number of values in one proof.
	MaxCommitments = 16

	minRounds = 6
	maxRounds = 10
)

var (
	ErrEmptyStatement      = errors.New("bulletproofs+: empty statement")
	ErrTooManyCommitments  = errors.New("bulletproofs+: too many commitments")
	ErrMismatchedWitness   = errors.New("bulletproofs+: values and blindings have different lengths")
	ErrInvalidScalar       = errors.New("bulletproofs+: invalid scalar encoding")
	ErrInvalidPoint        = errors.New("bulletproofs+: invalid point encoding")
	ErrInvalidProof        = errors.New("bulletproofs+: invalid proof")
	ErrUnexpectedEnd       = errors.New("bulletproofs+: truncated proof")
	ErrTrailingData        = errors.New("bulletproofs+: trailing proof data")
	errZeroChallenge       = errors.New("bulletproofs+: zero transcript challenge")
	errWitnessRelationship = errors.New("bulletproofs+: witness does not match statement")
)

// Scalar is a canonical little-endian scalar modulo the Ed25519 subgroup
// order. It is used for Pedersen commitment blindings.
type Scalar [32]byte

// EncodedPoint is a canonical compressed Edwards25519 point.
type EncodedPoint [32]byte

// Commitment is a canonical compressed prime-order Edwards25519 point.
type Commitment [32]byte

// Proof is a Monero-compatible Bulletproofs+ proof. Commitments are external
// to the proof, as they are in Monero's transaction encoding.
type Proof struct {
	A  EncodedPoint
	A1 EncodedPoint
	B  EncodedPoint

	R1 Scalar
	S1 Scalar
	D1 Scalar

	L []EncodedPoint
	R []EncodedPoint
}

// BatchItem associates a proof with the commitments it proves.
type BatchItem struct {
	Proof       *Proof
	Commitments []Commitment
}

// ParseScalar validates and copies a canonical scalar.
func ParseScalar(src []byte) (Scalar, error) {
	var out Scalar
	if len(src) != len(out) {
		return out, fmt.Errorf("%w: length %d", ErrInvalidScalar, len(src))
	}
	copy(out[:], src)
	if _, err := decodeScalar(out); err != nil {
		return Scalar{}, err
	}
	return out, nil
}

func paddedCommitments(n int) (int, error) {
	if n == 0 {
		return 0, ErrEmptyStatement
	}
	if n > MaxCommitments {
		return 0, fmt.Errorf("%w: %d > %d", ErrTooManyCommitments, n, MaxCommitments)
	}
	m := 1
	for m < n {
		m <<= 1
	}
	return m, nil
}

func expectedRounds(commitments int) (int, error) {
	m, err := paddedCommitments(commitments)
	if err != nil {
		return 0, err
	}
	rounds := minRounds
	for m > 1 {
		rounds++
		m >>= 1
	}
	return rounds, nil
}
