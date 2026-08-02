package bulletproofs

import "filippo.io/edwards25519"

type rangeComputation struct {
	y                edwards25519.Scalar
	z                edwards25519.Scalar
	dDescendingPlusZ scalarVector
	yMNPlusOne       edwards25519.Scalar
	zPowers          scalarVector
	aHat             *edwards25519.Point
}

func computeRange(
	commitments []*edwards25519.Point,
	transcript *edwards25519.Scalar,
	aEncoded EncodedPoint,
) (*rangeComputation, error) {
	m, err := paddedCommitments(len(commitments))
	if err != nil {
		return nil, err
	}
	cache := loadGenerators()
	aTransmitted, err := decodeCanonicalPoint(aEncoded)
	if err != nil {
		return nil, err
	}

	y := transcriptPoint(transcript, aEncoded)
	if scalarIsZero(y) {
		return nil, errZeroChallenge
	}
	z := hashToScalar(y.Bytes())
	if scalarIsZero(z) {
		return nil, errZeroChallenge
	}
	transcript.Set(z)

	mn := m * RangeBits
	zSquared := new(edwards25519.Scalar).Multiply(z, z)
	zPowers := make(scalarVector, m)
	zPowers[0].Set(zSquared)
	for j := 1; j < m; j++ {
		zPowers[j].Multiply(&zPowers[j-1], zSquared)
	}

	d := make(scalarVector, mn)
	twos := twoPowers()
	for j := range m {
		for i := range RangeBits {
			d[j*RangeBits+i].Multiply(&zPowers[j], &twos[i])
		}
	}
	dSum := d.sum()

	yAscending := scalarPowers(y, mn)
	ySum := yAscending.sum()
	dDescendingPlusZ := make(scalarVector, mn)
	for i := range dDescendingPlusZ {
		dDescendingPlusZ[i].Multiply(&d[i], &yAscending[mn-1-i])
		dDescendingPlusZ[i].Add(&dDescendingPlusZ[i], z)
	}
	yMNPlusOne := new(edwards25519.Scalar).Multiply(&yAscending[mn-1], y)

	commitmentScalars := make([]*edwards25519.Scalar, len(commitments))
	for i := range commitments {
		commitmentScalars[i] = &zPowers[i]
	}
	commitmentAccumulator := multiScalar(false, commitmentScalars, commitments)

	scalars := make([]*edwards25519.Scalar, 0, 2*mn+3)
	points := make([]*edwards25519.Point, 0, 2*mn+3)
	scalars = append(scalars, scalarOne)
	points = append(points, scalePoint(aTransmitted, scalarEight))
	negZ := new(edwards25519.Scalar).Negate(z)
	for i := range mn {
		scalars = append(scalars, negZ)
		points = append(points, cache.g[i])
		scalars = append(scalars, &dDescendingPlusZ[i])
		points = append(points, cache.h[i])
	}
	scalars = append(scalars, yMNPlusOne)
	points = append(points, commitmentAccumulator)

	// ySum*z - dSum*y^(MN+1)*z - ySum*z^2
	tmp := new(edwards25519.Scalar).Multiply(ySum, z)
	tmp.Subtract(tmp, new(edwards25519.Scalar).Multiply(
		new(edwards25519.Scalar).Multiply(dSum, yMNPlusOne), z,
	))
	tmp.Subtract(tmp, new(edwards25519.Scalar).Multiply(ySum, zSquared))
	scalars = append(scalars, tmp)
	points = append(points, cache.value)

	return &rangeComputation{
		y:                *y,
		z:                *z,
		dDescendingPlusZ: dDescendingPlusZ,
		yMNPlusOne:       *yMNPlusOne,
		zPowers:          zPowers,
		aHat:             multiScalar(false, scalars, points),
	}, nil
}
