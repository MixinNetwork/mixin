package bulletproofs

import (
	cryptorand "crypto/rand"
	"fmt"
	"io"

	"filippo.io/edwards25519"
)

type scalarPoint struct {
	scalar edwards25519.Scalar
	point  edwards25519.Point
}

type batchVerifier struct {
	amount edwards25519.Scalar
	blind  edwards25519.Scalar
	g      scalarVector
	h      scalarVector
	other  []scalarPoint
}

// Verify checks a single proof without randomized batching.
func (proof *Proof) Verify(commitments []Commitment) bool {
	if proof == nil {
		return false
	}
	verifier := new(batchVerifier)
	if !verifier.accumulate(BatchItem{Proof: proof, Commitments: commitments}, scalarOne) {
		return false
	}
	return verifier.verify()
}

// VerifyBatch verifies all items in one randomized multiscalar equation. The
// batch weights are drawn from crypto/rand, which guarantees they are
// unpredictable and independent of the proofs. A randomness failure is
// returned as an error, while a malformed or algebraically invalid proof
// returns false.
func VerifyBatch(items []BatchItem) (bool, error) {
	return verifyBatch(items, cryptorand.Reader)
}

// verifyBatch is the reader-taking variant, kept internal so tests can use
// deterministic batch weights. random must be non-nil and must provide
// unpredictable bytes independent of the proofs; never expose it to callers
// that could supply a seeded or otherwise predictable stream.
func verifyBatch(items []BatchItem, random io.Reader) (bool, error) {
	if len(items) == 0 {
		return false, ErrEmptyStatement
	}
	if random == nil {
		return false, fmt.Errorf("bulletproofs+: nil batch randomness")
	}
	verifier := new(batchVerifier)
	for i, item := range items {
		weight, err := randomScalar(random)
		if err != nil {
			return false, fmt.Errorf("bulletproofs+: batch weight %d: %w", i, err)
		}
		if !verifier.accumulate(item, weight) {
			return false, nil
		}
	}
	return verifier.verify(), nil
}

func (verifier *batchVerifier) addOther(s *edwards25519.Scalar, p *edwards25519.Point) {
	var pair scalarPoint
	pair.scalar.Set(s)
	pair.point.Set(p)
	verifier.other = append(verifier.other, pair)
}

func (verifier *batchVerifier) accumulate(item BatchItem, weight *edwards25519.Scalar) bool {
	if item.Proof == nil || item.Proof.validateEncoding() != nil {
		return false
	}
	rounds, err := expectedRounds(len(item.Commitments))
	if err != nil || len(item.Proof.L) != rounds || len(item.Proof.R) != rounds {
		return false
	}
	commitmentPointers := make([]*edwards25519.Point, len(item.Commitments))
	for i, encoded := range item.Commitments {
		point, err := decodeCommitment(encoded)
		if err != nil {
			return false
		}
		commitmentPointers[i] = point
	}
	transcript := startTranscript(commitmentPointers)
	rangeData, err := computeRange(commitmentPointers, transcript, item.Proof.A)
	if err != nil {
		return false
	}
	return verifier.accumulateWeighted(rangeData.aHat, &rangeData.y, transcript, item.Proof, weight)
}

func (verifier *batchVerifier) accumulateWeighted(
	statement *edwards25519.Point,
	y *edwards25519.Scalar,
	transcript *edwards25519.Scalar,
	proof *Proof,
	weight *edwards25519.Scalar,
) bool {
	n := 1 << len(proof.L)
	if n < RangeBits || n > MaxCommitments*RangeBits {
		return false
	}

	left := make([]edwards25519.Point, len(proof.L))
	right := make([]edwards25519.Point, len(proof.R))
	challenges := make([]edwards25519.Scalar, len(proof.L))
	challengeInverses := make([]edwards25519.Scalar, len(proof.L))
	for i := range proof.L {
		leftPoint, err := decodeCanonicalPoint(proof.L[i])
		if err != nil {
			return false
		}
		rightPoint, err := decodeCanonicalPoint(proof.R[i])
		if err != nil {
			return false
		}
		left[i].MultByCofactor(leftPoint)
		right[i].MultByCofactor(rightPoint)

		challenge := transcriptPoints(transcript, proof.L[i], proof.R[i])
		if scalarIsZero(challenge) {
			return false
		}
		challenges[i].Set(challenge)
		challengeInverses[i].Invert(challenge)
		transcript.Set(challenge)
	}

	a1Transmitted, err := decodeCanonicalPoint(proof.A1)
	if err != nil {
		return false
	}
	bTransmitted, err := decodeCanonicalPoint(proof.B)
	if err != nil {
		return false
	}
	a1 := edwards25519.NewIdentityPoint().MultByCofactor(a1Transmitted)
	b := edwards25519.NewIdentityPoint().MultByCofactor(bTransmitted)
	e := transcriptPoints(transcript, proof.A1, proof.B)
	if scalarIsZero(e) {
		return false
	}
	rAnswer, err := decodeScalar(proof.R1)
	if err != nil {
		return false
	}
	sAnswer, err := decodeScalar(proof.S1)
	if err != nil {
		return false
	}
	deltaAnswer, err := decodeScalar(proof.D1)
	if err != nil {
		return false
	}

	eSquared := new(edwards25519.Scalar).Multiply(e, e)
	negativeESquaredWeight := new(edwards25519.Scalar).Negate(eSquared)
	negativeESquaredWeight.Multiply(negativeESquaredWeight, weight)
	verifier.addOther(negativeESquaredWeight, statement)

	for i := range left {
		leftScalar := new(edwards25519.Scalar).Multiply(&challenges[i], &challenges[i])
		leftScalar.Multiply(leftScalar, negativeESquaredWeight)
		verifier.addOther(leftScalar, &left[i])

		rightScalar := new(edwards25519.Scalar).Multiply(&challengeInverses[i], &challengeInverses[i])
		rightScalar.Multiply(rightScalar, negativeESquaredWeight)
		verifier.addOther(rightScalar, &right[i])
	}

	products := challengeProducts(challenges, challengeInverses)
	if len(products) != n {
		return false
	}
	for len(verifier.g) < n {
		verifier.g = append(verifier.g, edwards25519.Scalar{})
		verifier.h = append(verifier.h, edwards25519.Scalar{})
	}

	yInverse := new(edwards25519.Scalar).Invert(y)
	yInversePowers := scalarPowers(yInverse, n-1)
	re := new(edwards25519.Scalar).Multiply(rAnswer, e)
	se := new(edwards25519.Scalar).Multiply(sAnswer, e)
	for i := range n {
		gScalar := new(edwards25519.Scalar).Multiply(&products[i], re)
		if i > 0 {
			gScalar.Multiply(gScalar, &yInversePowers[i-1])
		}
		gScalar.Multiply(gScalar, weight)
		verifier.g[i].Add(&verifier.g[i], gScalar)

		hScalar := new(edwards25519.Scalar).Multiply(&products[n-1-i], se)
		hScalar.Multiply(hScalar, weight)
		verifier.h[i].Add(&verifier.h[i], hScalar)
	}

	a1Scalar := new(edwards25519.Scalar).Negate(e)
	a1Scalar.Multiply(a1Scalar, weight)
	verifier.addOther(a1Scalar, a1)

	amountScalar := new(edwards25519.Scalar).Multiply(rAnswer, y)
	amountScalar.Multiply(amountScalar, sAnswer)
	amountScalar.Multiply(amountScalar, weight)
	verifier.amount.Add(&verifier.amount, amountScalar)

	blindScalar := new(edwards25519.Scalar).Multiply(deltaAnswer, weight)
	verifier.blind.Add(&verifier.blind, blindScalar)

	bScalar := new(edwards25519.Scalar).Negate(weight)
	verifier.addOther(bScalar, b)
	return true
}

func challengeProducts(challenges, inverses []edwards25519.Scalar) scalarVector {
	if len(challenges) != len(inverses) {
		return nil
	}
	if len(challenges) == 0 {
		return scalarVector{*new(edwards25519.Scalar).Set(scalarOne)}
	}
	products := make(scalarVector, 1<<len(challenges))
	products[0].Set(&inverses[0])
	products[1].Set(&challenges[0])
	for j := 1; j < len(challenges); j++ {
		last := (1 << (j + 1)) - 1
		for slot := last; slot >= 1; slot -= 2 {
			parent := slot / 2
			products[slot].Multiply(&products[parent], &challenges[j])
			products[slot-1].Multiply(&products[parent], &inverses[j])
		}
	}
	return products
}

func (verifier *batchVerifier) verify() bool {
	cache := loadGenerators()
	capacity := 2 + len(verifier.g) + len(verifier.h) + len(verifier.other)
	scalars := make([]*edwards25519.Scalar, 0, capacity)
	points := make([]*edwards25519.Point, 0, capacity)
	scalars = append(scalars, &verifier.amount, &verifier.blind)
	points = append(points, cache.value, edwards25519.NewGeneratorPoint())
	for i := range verifier.g {
		scalars = append(scalars, &verifier.g[i])
		points = append(points, cache.g[i])
	}
	for i := range verifier.h {
		scalars = append(scalars, &verifier.h[i])
		points = append(points, cache.h[i])
	}
	for i := range verifier.other {
		scalars = append(scalars, &verifier.other[i].scalar)
		points = append(points, &verifier.other[i].point)
	}
	return multiScalar(false, scalars, points).Equal(edwards25519.NewIdentityPoint()) == 1
}
