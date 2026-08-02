package bulletproofs

import "filippo.io/edwards25519"

type scalarVector []edwards25519.Scalar

func (v scalarVector) clone() scalarVector {
	result := make(scalarVector, len(v))
	copy(result, v)
	return result
}

func (v scalarVector) subtractScalar(s *edwards25519.Scalar) scalarVector {
	for i := range v {
		v[i].Subtract(&v[i], s)
	}
	return v
}

func (v scalarVector) multiplyScalar(s *edwards25519.Scalar) scalarVector {
	for i := range v {
		v[i].Multiply(&v[i], s)
	}
	return v
}

func (v scalarVector) addVector(other scalarVector) scalarVector {
	if len(v) != len(other) {
		panic("bulletproofs+: scalar-vector length mismatch")
	}
	for i := range v {
		v[i].Add(&v[i], &other[i])
	}
	return v
}

func (v scalarVector) addVectorTimes(other scalarVector, s *edwards25519.Scalar) scalarVector {
	if len(v) != len(other) {
		panic("bulletproofs+: scalar-vector length mismatch")
	}
	for i := range v {
		v[i].MultiplyAdd(&other[i], s, &v[i])
	}
	return v
}

func (v scalarVector) sum() *edwards25519.Scalar {
	result := edwards25519.NewScalar()
	for i := range v {
		result.Add(result, &v[i])
	}
	return result
}

func weightedInnerProduct(a, b, y scalarVector) *edwards25519.Scalar {
	if len(a) != len(b) || len(a) != len(y) {
		panic("bulletproofs+: weighted inner-product length mismatch")
	}
	result := edwards25519.NewScalar()
	tmp := edwards25519.NewScalar()
	for i := range a {
		tmp.Multiply(&a[i], &b[i])
		result.MultiplyAdd(tmp, &y[i], result)
	}
	return result
}

func decompose(value uint64) scalarVector {
	bits := [2]edwards25519.Scalar{*scalarZero, *scalarOne}
	result := make(scalarVector, RangeBits)
	for i := range result {
		result[i].Set(&bits[value&1])
		value >>= 1
	}
	return result
}

func twoPowers() scalarVector {
	result := make(scalarVector, RangeBits)
	result[0].Set(scalarOne)
	for i := 1; i < len(result); i++ {
		result[i].Add(&result[i-1], &result[i-1])
	}
	return result
}

func multiScalar(constantTime bool, scalars []*edwards25519.Scalar, points []*edwards25519.Point) *edwards25519.Point {
	if len(scalars) != len(points) {
		panic("bulletproofs+: multiscalar length mismatch")
	}
	if constantTime {
		return edwards25519.NewIdentityPoint().MultiScalarMult(scalars, points)
	}
	return edwards25519.NewIdentityPoint().VarTimeMultiScalarMult(scalars, points)
}
