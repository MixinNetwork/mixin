package crypto

import (
	"crypto/sha512"
	"encoding/binary"
	"fmt"

	"filippo.io/edwards25519"
)

const (
	aggregateCoefficientDomain = "mixin-aggregate-coefficient-v1"
	aggregateNonceDomain       = "mixin-aggregate-nonce-v1"
)

type aggregateSigner struct {
	index  int
	public *Key
	point  *edwards25519.Point
}

func collectAggregateSigners(publics []*Key, signers []int) ([]aggregateSigner, []byte, error) {
	if len(signers) == 0 {
		return nil, nil, fmt.Errorf("empty aggregation signers")
	}
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
		if publics[i] == nil {
			return nil, nil, fmt.Errorf("nil aggregation public key %d", i)
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

func aggregateWeightedPublicKey(publics []*Key, signers []int) (Key, []*edwards25519.Scalar, []byte, error) {
	selected, transcript, err := collectAggregateSigners(publics, signers)
	if err != nil {
		return Key{}, nil, nil, err
	}

	P := edwards25519.NewIdentityPoint()
	coefficients := make([]*edwards25519.Scalar, 0, len(selected))
	for _, signer := range selected {
		coeff, err := aggregateCoefficient(transcript, signer)
		if err != nil {
			return Key{}, nil, nil, err
		}
		weighted := edwards25519.NewIdentityPoint().ScalarMult(coeff, signer.point)
		P = P.Add(P, weighted)
		coefficients = append(coefficients, coeff)
	}

	var key Key
	copy(key[:], P.Bytes())
	return key, coefficients, transcript, nil
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

// AggregateSign produces a MuSig-style aggregate Schnorr signature over the
// given signers and message. Each nonce is derived from the private key, at
// least 32 bytes of caller-supplied entropy, the complete signer transcript,
// aggregate public key, signer index, and message. Binding all of these values
// prevents nonce reuse across changed signer sets and prevents disclosure of
// the auxiliary seed from exposing a private key.
func AggregateSign(privKeys []*Key, publics []*Key, signers []int, seed []byte, message Hash) (*Signature, error) {
	if len(privKeys) != len(signers) {
		return nil, fmt.Errorf("invalid aggregation private keys count %d/%d", len(privKeys), len(signers))
	}
	if len(seed) < 32 {
		return nil, fmt.Errorf("invalid aggregation seed size %d", len(seed))
	}

	A, coefficients, transcript, err := aggregateWeightedPublicKey(publics, signers)
	if err != nil {
		return nil, fmt.Errorf("AggregateSign aggregateWeightedPublicKey %v", err)
	}

	P := edwards25519.NewIdentityPoint()
	randoms := make([]*edwards25519.Scalar, 0, len(signers))
	privateScalars := make([]*edwards25519.Scalar, 0, len(privKeys))
	for i, signer := range signers {
		if signer > 0xFFFF {
			return nil, fmt.Errorf("invalid aggregation signer index %d", signer)
		}
		private := privKeys[i]
		if private == nil {
			return nil, fmt.Errorf("nil aggregation private key %d", i)
		}
		y, err := edwards25519.NewScalar().SetCanonicalBytes(private[:])
		if err != nil {
			return nil, err
		}
		if private.Public() != *publics[signer] {
			return nil, fmt.Errorf("aggregation private key %d does not match signer %d", i, signer)
		}
		privateScalars = append(privateScalars, y)

		var digest [64]byte
		var index [4]byte
		binary.BigEndian.PutUint32(index[:], uint32(signer))
		h := sha512.New()
		h.Write([]byte(aggregateNonceDomain))
		h.Write(private[:])
		h.Write(binary.BigEndian.AppendUint32(nil, uint32(len(seed))))
		h.Write(seed)
		h.Write(transcript)
		h.Write(A[:])
		h.Write(index[:])
		h.Write(message[:])
		h.Sum(digest[:0])
		z, err := edwards25519.NewScalar().SetUniformBytes(digest[:])
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
	for i, y := range privateScalars {
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
	if sig == nil {
		return fmt.Errorf("AggregateVerify nil signature")
	}
	A, _, _, err := aggregateWeightedPublicKey(publics, signers)
	if err != nil {
		return fmt.Errorf("AggregateVerify aggregateWeightedPublicKey %v", err)
	}
	if !A.Verify(message, *sig) {
		return fmt.Errorf("AggregateVerify signature verify failed")
	}
	return nil
}
