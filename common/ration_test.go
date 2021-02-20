package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRation(t *testing.T) {
	assert := assert.New(t)

	x := NewInteger(1)
	y := NewInteger(3)
	assert.Equal("1.00000000", x.String())
	assert.Equal("3.00000000", y.String())

	r := x.Ration(y)
	assert.Equal("1.00000000", x.String())
	assert.Equal("3.00000000", y.String())
	assert.Equal(int64(100000000), r.x.Int64())
	assert.Equal(int64(300000000), r.y.Int64())

	p := r.Product(y)
	assert.Equal("1.00000000", p.String())
	assert.Equal("1.00000000", x.String())
	assert.Equal("3.00000000", y.String())
	assert.Equal(int64(100000000), r.x.Int64())
	assert.Equal(int64(300000000), r.y.Int64())

	a := p.Ration(y)
	assert.Equal("1.00000000", p.String())
	assert.Equal("1.00000000", x.String())
	assert.Equal("3.00000000", y.String())
	assert.Equal(int64(100000000), r.x.Int64())
	assert.Equal(int64(300000000), r.y.Int64())
	assert.Equal(int64(100000000), a.x.Int64())
	assert.Equal(int64(300000000), a.y.Int64())

	cmp := r.Cmp(a)
	assert.Equal(0, cmp)
	assert.Equal("1.00000000", p.String())
	assert.Equal("1.00000000", x.String())
	assert.Equal("3.00000000", y.String())
	assert.Equal(int64(100000000), r.x.Int64())
	assert.Equal(int64(300000000), r.y.Int64())
	assert.Equal(int64(100000000), a.x.Int64())
	assert.Equal(int64(300000000), a.y.Int64())

	a = y.Ration(p)
	cmp = r.Cmp(a)
	assert.Equal(-1, cmp)
	assert.Equal("1.00000000", p.String())
	assert.Equal("1.00000000", x.String())
	assert.Equal("3.00000000", y.String())
	assert.Equal(int64(100000000), r.x.Int64())
	assert.Equal(int64(300000000), r.y.Int64())
	assert.Equal(int64(300000000), a.x.Int64())
	assert.Equal(int64(100000000), a.y.Int64())
}
