package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRation(t *testing.T) {
	require := require.New(t)

	x := NewInteger(1)
	y := NewInteger(3)
	require.Equal("1.00000000", x.String())
	require.Equal("3.00000000", y.String())

	r := x.Ration(y)
	require.Equal("1.00000000", x.String())
	require.Equal("3.00000000", y.String())
	require.Equal(int64(100000000), r.x.Int64())
	require.Equal(int64(300000000), r.y.Int64())

	p := r.Product(y)
	require.Equal("1.00000000", p.String())
	require.Equal("1.00000000", x.String())
	require.Equal("3.00000000", y.String())
	require.Equal(int64(100000000), r.x.Int64())
	require.Equal(int64(300000000), r.y.Int64())

	a := p.Ration(y)
	require.Equal("1.00000000", p.String())
	require.Equal("1.00000000", x.String())
	require.Equal("3.00000000", y.String())
	require.Equal(int64(100000000), r.x.Int64())
	require.Equal(int64(300000000), r.y.Int64())
	require.Equal(int64(100000000), a.x.Int64())
	require.Equal(int64(300000000), a.y.Int64())

	cmp := r.Cmp(a)
	require.Equal(0, cmp)
	require.Equal("1.00000000", p.String())
	require.Equal("1.00000000", x.String())
	require.Equal("3.00000000", y.String())
	require.Equal(int64(100000000), r.x.Int64())
	require.Equal(int64(300000000), r.y.Int64())
	require.Equal(int64(100000000), a.x.Int64())
	require.Equal(int64(300000000), a.y.Int64())

	a = y.Ration(p)
	cmp = r.Cmp(a)
	require.Equal(-1, cmp)
	require.Equal("1.00000000", p.String())
	require.Equal("1.00000000", x.String())
	require.Equal("3.00000000", y.String())
	require.Equal(int64(100000000), r.x.Int64())
	require.Equal(int64(300000000), r.y.Int64())
	require.Equal(int64(300000000), a.x.Int64())
	require.Equal(int64(100000000), a.y.Int64())
}
