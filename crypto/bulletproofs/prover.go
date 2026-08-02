package bulletproofs

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"filippo.io/edwards25519"
)

// Prove constructs one aggregated Monero-compatible Bulletproofs+ proof and
// returns the corresponding ordinary Pedersen commitments. Randomness defaults
// to crypto/rand.Reader when random is nil.
func Prove(values []uint64, blindings []Scalar, random io.Reader) (*Proof, []Commitment, error) {
	if len(values) != len(blindings) {
		return nil, nil, ErrMismatchedWitness
	}
	m, err := paddedCommitments(len(values))
	if err != nil {
		return nil, nil, err
	}
	if random == nil {
		random = rand.Reader
	}

	blinds := make([]edwards25519.Scalar, len(blindings))
	commitments := make([]Commitment, len(values))
	commitmentPoints := make([]*edwards25519.Point, len(values))
	cache := loadGenerators()
	for i := range values {
		blind, err := decodeScalar(blindings[i])
		if err != nil {
			return nil, nil, fmt.Errorf("blinding %d: %w", i, err)
		}
		blinds[i].Set(blind)
		amount := scalarFromUint64(values[i])
		commitmentPoints[i] = multiScalar(true,
			[]*edwards25519.Scalar{&blinds[i], amount},
			[]*edwards25519.Point{edwards25519.NewGeneratorPoint(), cache.value},
		)
		copy(commitments[i][:], commitmentPoints[i].Bytes())
	}

	mn := m * RangeBits
	aL := make(scalarVector, mn)
	for j := range m {
		var bits scalarVector
		if j < len(values) {
			bits = decompose(values[j])
		} else {
			bits = decompose(0)
		}
		copy(aL[j*RangeBits:(j+1)*RangeBits], bits)
	}
	aR := aL.clone().subtractScalar(scalarOne)

	for {
		proof, err := proveAttempt(commitmentPoints, blinds, aL, aR, random)
		if errors.Is(err, errZeroChallenge) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		return proof, commitments, nil
	}
}

func proveAttempt(
	commitments []*edwards25519.Point,
	blinds []edwards25519.Scalar,
	aL, aR scalarVector,
	random io.Reader,
) (*Proof, error) {
	cache := loadGenerators()
	transcript := startTranscript(commitments)
	alpha, err := randomScalar(random)
	if err != nil {
		return nil, err
	}

	scalars := make([]*edwards25519.Scalar, 0, 2*len(aL)+1)
	points := make([]*edwards25519.Point, 0, 2*len(aL)+1)
	for i := range aL {
		scalars = append(scalars, &aL[i])
		points = append(points, cache.g[i])
	}
	for i := range aR {
		scalars = append(scalars, &aR[i])
		points = append(points, cache.h[i])
	}
	scalars = append(scalars, alpha)
	points = append(points, edwards25519.NewGeneratorPoint())
	aActual := multiScalar(true, scalars, points)
	aEncoded := encodePoint(scalePoint(aActual, scalarInvEight))

	computation, err := computeRange(commitments, transcript, aEncoded)
	if err != nil {
		return nil, err
	}
	a := aL.clone().subtractScalar(&computation.z)
	b := aR.clone().addVector(computation.dDescendingPlusZ)
	alphaHat := new(edwards25519.Scalar).Set(alpha)
	for j := range blinds {
		term := new(edwards25519.Scalar).Multiply(&computation.zPowers[j], &blinds[j])
		term.Multiply(term, &computation.yMNPlusOne)
		alphaHat.Add(alphaHat, term)
	}

	wip, err := proveWeightedInnerProduct(
		computation.aHat, &computation.y, a, b, alphaHat, transcript, random,
	)
	if err != nil {
		return nil, err
	}
	return &Proof{
		A:  aEncoded,
		A1: wip.a1,
		B:  wip.b,
		R1: wip.r,
		S1: wip.s,
		D1: wip.delta,
		L:  wip.left,
		R:  wip.right,
	}, nil
}

type weightedProof struct {
	left  []EncodedPoint
	right []EncodedPoint
	a1    EncodedPoint
	b     EncodedPoint
	r     Scalar
	s     Scalar
	delta Scalar
}

func proveWeightedInnerProduct(
	statement *edwards25519.Point,
	y *edwards25519.Scalar,
	a, b scalarVector,
	alpha *edwards25519.Scalar,
	transcript *edwards25519.Scalar,
	random io.Reader,
) (*weightedProof, error) {
	if len(a) == 0 || len(a) != len(b) || len(a)&(len(a)-1) != 0 {
		return nil, ErrInvalidProof
	}
	cache := loadGenerators()
	g := make([]*edwards25519.Point, len(a))
	h := make([]*edwards25519.Point, len(a))
	for i := range a {
		g[i] = edwards25519.NewIdentityPoint().Set(cache.g[i])
		h[i] = edwards25519.NewIdentityPoint().Set(cache.h[i])
	}
	yVector := scalarPowers(y, len(a))

	// Check the internal witness relation before emitting a proof.
	scalars := make([]*edwards25519.Scalar, 0, 2*len(a)+2)
	points := make([]*edwards25519.Point, 0, 2*len(a)+2)
	for i := range a {
		scalars = append(scalars, &a[i])
		points = append(points, g[i])
		scalars = append(scalars, &b[i])
		points = append(points, h[i])
	}
	inner := weightedInnerProduct(a, b, yVector)
	scalars = append(scalars, inner, alpha)
	points = append(points, cache.value, edwards25519.NewGeneratorPoint())
	if multiScalar(true, scalars, points).Equal(statement) != 1 {
		return nil, errWitnessRelationship
	}

	a = a.clone()
	b = b.clone()
	alphaFolded := new(edwards25519.Scalar).Set(alpha)
	result := &weightedProof{}

	for len(g) > 1 {
		n := len(g) / 2
		a1, a2 := a[:n], a[n:]
		b1, b2 := b[:n], b[n:]
		g1, g2 := g[:n], g[n:]
		h1, h2 := h[:n], h[n:]
		yRound := yVector[:n]
		yN := &yRound[n-1]
		yNInv := new(edwards25519.Scalar).Invert(yN)

		dLeft, err := randomScalar(random)
		if err != nil {
			return nil, err
		}
		dRight, err := randomScalar(random)
		if err != nil {
			return nil, err
		}
		cLeft := weightedInnerProduct(a1, b2, yRound)
		a2Weighted := a2.clone().multiplyScalar(yN)
		cRight := weightedInnerProduct(a2Weighted, b1, yRound)

		scalars = scalars[:0]
		points = points[:0]
		a1Weighted := a1.clone().multiplyScalar(yNInv)
		for i := 0; i < n; i++ {
			scalars = append(scalars, &a1Weighted[i], &b2[i])
			points = append(points, g2[i], h1[i])
		}
		scalars = append(scalars, cLeft, dLeft)
		points = append(points, cache.value, edwards25519.NewGeneratorPoint())
		left := encodePoint(scalePoint(multiScalar(true, scalars, points), scalarInvEight))

		scalars = scalars[:0]
		points = points[:0]
		for i := 0; i < n; i++ {
			scalars = append(scalars, &a2Weighted[i], &b1[i])
			points = append(points, g1[i], h2[i])
		}
		scalars = append(scalars, cRight, dRight)
		points = append(points, cache.value, edwards25519.NewGeneratorPoint())
		right := encodePoint(scalePoint(multiScalar(true, scalars, points), scalarInvEight))

		challenge := transcriptPoints(transcript, left, right)
		if scalarIsZero(challenge) {
			return nil, errZeroChallenge
		}
		transcript.Set(challenge)
		challengeInv := new(edwards25519.Scalar).Invert(challenge)
		challengeSquared := new(edwards25519.Scalar).Multiply(challenge, challenge)
		challengeInvSquared := new(edwards25519.Scalar).Multiply(challengeInv, challengeInv)

		gFolded := make([]*edwards25519.Point, n)
		hFolded := make([]*edwards25519.Point, n)
		challengeYInv := new(edwards25519.Scalar).Multiply(challenge, yNInv)
		for i := 0; i < n; i++ {
			gFolded[i] = multiScalar(true,
				[]*edwards25519.Scalar{challengeInv, challengeYInv},
				[]*edwards25519.Point{g1[i], g2[i]},
			)
			hFolded[i] = multiScalar(true,
				[]*edwards25519.Scalar{challenge, challengeInv},
				[]*edwards25519.Point{h1[i], h2[i]},
			)
		}

		aFolded := a1.clone().multiplyScalar(challenge)
		aFolded.addVectorTimes(a2, new(edwards25519.Scalar).Multiply(yN, challengeInv))
		bFolded := b1.clone().multiplyScalar(challengeInv)
		bFolded.addVectorTimes(b2, challenge)
		alphaFolded.Add(alphaFolded,
			new(edwards25519.Scalar).Multiply(dLeft, challengeSquared),
		)
		alphaFolded.Add(alphaFolded,
			new(edwards25519.Scalar).Multiply(dRight, challengeInvSquared),
		)

		result.left = append(result.left, left)
		result.right = append(result.right, right)
		g, h = gFolded, hFolded
		a, b = aFolded, bFolded
		yVector = yRound
	}

	r, err := randomScalar(random)
	if err != nil {
		return nil, err
	}
	s, err := randomScalar(random)
	if err != nil {
		return nil, err
	}
	delta, err := randomScalar(random)
	if err != nil {
		return nil, err
	}
	eta, err := randomScalar(random)
	if err != nil {
		return nil, err
	}

	ry := new(edwards25519.Scalar).Multiply(r, &yVector[0])
	amountTerm := new(edwards25519.Scalar).Multiply(ry, &b[0])
	amountTerm.Add(amountTerm,
		new(edwards25519.Scalar).Multiply(
			new(edwards25519.Scalar).Multiply(s, &yVector[0]), &a[0],
		),
	)
	a1Actual := multiScalar(true,
		[]*edwards25519.Scalar{r, s, amountTerm, delta},
		[]*edwards25519.Point{g[0], h[0], cache.value, edwards25519.NewGeneratorPoint()},
	)
	a1Encoded := encodePoint(scalePoint(a1Actual, scalarInvEight))

	bAmount := new(edwards25519.Scalar).Multiply(ry, s)
	bActual := multiScalar(true,
		[]*edwards25519.Scalar{bAmount, eta},
		[]*edwards25519.Point{cache.value, edwards25519.NewGeneratorPoint()},
	)
	bEncoded := encodePoint(scalePoint(bActual, scalarInvEight))

	e := transcriptPoints(transcript, a1Encoded, bEncoded)
	if scalarIsZero(e) {
		return nil, errZeroChallenge
	}
	eSquared := new(edwards25519.Scalar).Multiply(e, e)
	rAnswer := new(edwards25519.Scalar).MultiplyAdd(&a[0], e, r)
	sAnswer := new(edwards25519.Scalar).MultiplyAdd(&b[0], e, s)
	deltaAnswer := new(edwards25519.Scalar).MultiplyAdd(delta, e, eta)
	deltaAnswer.Add(deltaAnswer,
		new(edwards25519.Scalar).Multiply(alphaFolded, eSquared),
	)

	result.a1 = a1Encoded
	result.b = bEncoded
	result.r = encodeScalar(rAnswer)
	result.s = encodeScalar(sAnswer)
	result.delta = encodeScalar(deltaAnswer)
	return result, nil
}
