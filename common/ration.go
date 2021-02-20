package common

import (
	"fmt"
	"math/big"
)

var OneRat RationalNumber

func init() {
	OneRat = NewInteger(1).Ration(NewInteger(1))
}

type RationalNumber struct {
	x big.Int
	y big.Int
}

func (x Integer) Ration(y Integer) (v RationalNumber) {
	if x.Sign() < 0 || y.Sign() <= 0 {
		panic(fmt.Sprint(x, y))
	}

	v.x.SetBytes(x.i.Bytes())
	v.y.SetBytes(y.i.Bytes())
	return
}

func (r RationalNumber) Product(x Integer) (v Integer) {
	if x.Sign() < 0 {
		panic(fmt.Sprint(x, r))
	}

	v.i.Mul(&x.i, &r.x)
	v.i.Div(&v.i, &r.y)
	return
}

func (r RationalNumber) Cmp(x RationalNumber) int {
	var v RationalNumber
	v.x.Mul(&r.x, &x.y)
	v.y.Mul(&r.y, &x.x)
	return v.x.Cmp(&v.y)
}
