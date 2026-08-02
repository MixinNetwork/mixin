package bulletproofs

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// MarshalBinary returns the standalone P2Pool-compatible encoding
// A || A1 || B || r1 || s1 || d1 || len(L) || L || len(R) || R. Monero's
// transaction encoding carries the two vector lengths in its container; use
// this encoding when the proof is stored independently.
func (proof Proof) MarshalBinary() ([]byte, error) {
	if err := proof.validateEncoding(); err != nil {
		return nil, err
	}
	capacity := 6*32 + 2 + (len(proof.L)+len(proof.R))*32
	result := make([]byte, 0, capacity)
	result = append(result, proof.A[:]...)
	result = append(result, proof.A1[:]...)
	result = append(result, proof.B[:]...)
	result = append(result, proof.R1[:]...)
	result = append(result, proof.S1[:]...)
	result = append(result, proof.D1[:]...)
	result = binary.AppendUvarint(result, uint64(len(proof.L)))
	for _, point := range proof.L {
		result = append(result, point[:]...)
	}
	result = binary.AppendUvarint(result, uint64(len(proof.R)))
	for _, point := range proof.R {
		result = append(result, point[:]...)
	}
	return result, nil
}

// UnmarshalBinary parses the strict standalone encoding. It rejects
// non-canonical varints, malformed point/scalar encodings, oversized vectors,
// truncation, and trailing bytes.
func (proof *Proof) UnmarshalBinary(data []byte) error {
	if proof == nil {
		return ErrInvalidProof
	}
	var parsed Proof
	offset := 0
	read32 := func(dst []byte) error {
		if len(data)-offset < 32 {
			return ErrUnexpectedEnd
		}
		copy(dst, data[offset:offset+32])
		offset += 32
		return nil
	}
	for _, dst := range [][]byte{
		parsed.A[:], parsed.A1[:], parsed.B[:],
		parsed.R1[:], parsed.S1[:], parsed.D1[:],
	} {
		if err := read32(dst); err != nil {
			return err
		}
	}

	leftCount, consumed, err := readCanonicalUvarint(data[offset:])
	if err != nil {
		return err
	}
	offset += consumed
	if leftCount < minRounds || leftCount > maxRounds {
		return fmt.Errorf("%w: invalid L length %d", ErrInvalidProof, leftCount)
	}
	parsed.L = make([]EncodedPoint, int(leftCount))
	for i := range parsed.L {
		if err := read32(parsed.L[i][:]); err != nil {
			return err
		}
	}

	rightCount, consumed, err := readCanonicalUvarint(data[offset:])
	if err != nil {
		return err
	}
	offset += consumed
	if rightCount != leftCount {
		return fmt.Errorf("%w: mismatched L/R lengths", ErrInvalidProof)
	}
	parsed.R = make([]EncodedPoint, int(rightCount))
	for i := range parsed.R {
		if err := read32(parsed.R[i][:]); err != nil {
			return err
		}
	}
	if offset != len(data) {
		return ErrTrailingData
	}
	if err := parsed.validateEncoding(); err != nil {
		return err
	}
	*proof = parsed
	return nil
}

func readCanonicalUvarint(data []byte) (uint64, int, error) {
	value, consumed := binary.Uvarint(data)
	if consumed == 0 {
		return 0, 0, ErrUnexpectedEnd
	}
	if consumed < 0 {
		return 0, 0, fmt.Errorf("%w: overflowing vector length", ErrInvalidProof)
	}
	var canonical [binary.MaxVarintLen64]byte
	size := binary.PutUvarint(canonical[:], value)
	if size != consumed || !bytes.Equal(canonical[:size], data[:consumed]) {
		return 0, 0, fmt.Errorf("%w: non-canonical vector length", ErrInvalidProof)
	}
	return value, consumed, nil
}

func (proof *Proof) validateEncoding() error {
	if proof == nil {
		return ErrInvalidProof
	}
	if len(proof.L) < minRounds || len(proof.L) > maxRounds || len(proof.L) != len(proof.R) {
		return fmt.Errorf("%w: invalid L/R lengths %d/%d", ErrInvalidProof, len(proof.L), len(proof.R))
	}
	for name, encoded := range map[string]EncodedPoint{
		"A": proof.A, "A1": proof.A1, "B": proof.B,
	} {
		if _, err := decodeCanonicalPoint(encoded); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidProof, name, err)
		}
	}
	for i := range proof.L {
		if _, err := decodeCanonicalPoint(proof.L[i]); err != nil {
			return fmt.Errorf("%w: L[%d]: %v", ErrInvalidProof, i, err)
		}
		if _, err := decodeCanonicalPoint(proof.R[i]); err != nil {
			return fmt.Errorf("%w: R[%d]: %v", ErrInvalidProof, i, err)
		}
	}
	for name, encoded := range map[string]Scalar{
		"r1": proof.R1, "s1": proof.S1, "d1": proof.D1,
	} {
		if _, err := decodeScalar(encoded); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrInvalidProof, name, err)
		}
	}
	return nil
}
