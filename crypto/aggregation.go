package crypto

import (
	"crypto/sha512"
	"encoding/binary"
	"fmt"

	"filippo.io/edwards25519"
)

const aggregateCoefficientDomain = "mixin-aggregate-coefficient-v1"

type aggregateSigner struct {
	index  int
	public *Key
	point  *edwards25519.Point
}

func collectAggregateSigners(publics []*Key, signers []int) ([]aggregateSigner, []byte, error) {
	selected := make([]aggregateSigner, 0, len(signers))
	transcript := binary.BigEndian.AppendUint32(nil, uint32(len(signers)))
	prev := -1
	for _, i := range signers {
		if i <= prev {
			return nil, nil, fmt.Errorf("invalid aggregation signer order %d <= %d", i, prev)
		}
		if i >= len(publics) {
			return nil, nil, fmt.Errorf("invalid aggregation signer index %d/%d", i, len(publics))
		}
		p, err := decodePoint(publics[i][:])
		if err != nil {
			return nil, nil, err
		}
		selected = append(selected, aggregateSigner{index: i, public: publics[i], point: p})
		transcript = binary.BigEndian.AppendUint32(transcript, uint32(i))
		transcript = append(transcript, publics[i][:]...)
		prev = i
	}
	return selected, transcript, nil
}

func aggregateCoefficient(transcript []byte, signer aggregateSigner) (*edwards25519.Scalar, error) {
	var idx [4]byte
	var digest [64]byte
	binary.BigEndian.PutUint32(idx[:], uint32(signer.index))

	h := sha512.New()
	h.Write([]byte(aggregateCoefficientDomain))
	h.Write(transcript)
	h.Write(idx[:])
	h.Write(signer.public[:])
	h.Sum(digest[:0])

	return edwards25519.NewScalar().SetUniformBytes(digest[:])
}

func aggregatePublicKey(publics []*Key, signers []int) (*Key, error) {
	selected, _, err := collectAggregateSigners(publics, signers)
	if err != nil {
		return nil, err
	}

	P := edwards25519.NewIdentityPoint()
	for _, signer := range selected {
		P = P.Add(P, signer.point)
	}

	var key Key
	copy(key[:], P.Bytes())
	return &key, nil
}

func aggregateWeightedPublicKey(publics []*Key, signers []int) (Key, []*edwards25519.Scalar, error) {
	selected, transcript, err := collectAggregateSigners(publics, signers)
	if err != nil {
		return Key{}, nil, err
	}

	P := edwards25519.NewIdentityPoint()
	coefficients := make([]*edwards25519.Scalar, 0, len(selected))
	for _, signer := range selected {
		coeff, err := aggregateCoefficient(transcript, signer)
		if err != nil {
			return Key{}, nil, err
		}
		weighted := edwards25519.NewIdentityPoint().ScalarMult(coeff, signer.point)
		P = P.Add(P, weighted)
		coefficients = append(coefficients, coeff)
	}

	var key Key
	copy(key[:], P.Bytes())
	return key, coefficients, nil
}

func aggregateChallenge(commitment, public []byte, message Hash) (*edwards25519.Scalar, error) {
	var digest [64]byte
	h := sha512.New()
	h.Write(commitment)
	h.Write(public)
	h.Write(message[:])
	h.Sum(digest[:0])
	return edwards25519.NewScalar().SetUniformBytes(digest[:])
}

func AggregateSign(privKeys []*Key, publics []*Key, signers []int, seed []byte, message Hash) (*Signature, error) {
	if len(privKeys) != len(signers) {
		return nil, fmt.Errorf("invalid aggregation private keys count %d/%d", len(privKeys), len(signers))
	}

	A, coefficients, err := aggregateWeightedPublicKey(publics, signers)
	if err != nil {
		return nil, fmt.Errorf("AggregateSign aggregateWeightedPublicKey %v", err)
	}

	P := edwards25519.NewIdentityPoint()
	randoms := make([]*edwards25519.Scalar, 0, len(signers))
	for _, signer := range signers {
		if signer > 0xFFFF {
			return nil, fmt.Errorf("invalid aggregation signer index %d", signer)
		}
		buf := binary.BigEndian.AppendUint16(seed, uint16(signer))
		s := Blake3Hash(append(buf, message[:]...))
		r := NewKeyFromSeed(append(s[:], s[:]...))
		z, err := edwards25519.NewScalar().SetCanonicalBytes(r[:])
		if err != nil {
			return nil, err
		}
		randoms = append(randoms, z)

		p := edwards25519.NewIdentityPoint().ScalarBaseMult(z)
		P = P.Add(P, p)
	}

	x, err := aggregateChallenge(P.Bytes(), A[:], message)
	if err != nil {
		return nil, err
	}

	S := edwards25519.NewScalar()
	for i, k := range privKeys {
		y, err := edwards25519.NewScalar().SetCanonicalBytes(k[:])
		if err != nil {
			return nil, err
		}
		weighted := edwards25519.NewScalar().Multiply(coefficients[i], y)
		s := edwards25519.NewScalar().MultiplyAdd(x, weighted, randoms[i])
		S = S.Add(S, s)
	}

	var sig Signature
	copy(sig[:32], P.Bytes())
	copy(sig[32:], S.Bytes())
	return &sig, nil
}

func AggregateVerify(sig *Signature, publics []*Key, signers []int, message Hash) error {
	A, _, err := aggregateWeightedPublicKey(publics, signers)
	if err != nil {
		return fmt.Errorf("AggregateVerify aggregateWeightedPublicKey %v", err)
	}
	if !A.Verify(message, *sig) {
		return fmt.Errorf("AggregateVerify signature verify failed")
	}
	return nil
}
