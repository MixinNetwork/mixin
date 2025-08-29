package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInteger(t *testing.T) {
	require := require.New(t)

	require.Equal("0.00000000", NewInteger(0).String())

	a := NewInteger(10000)
	b := NewIntegerFromString("10000")
	require.Equal(0, a.Cmp(b))

	c := a.Add(b)
	require.Equal("20000.00000000", c.String())
	j, err := c.MarshalJSON()
	require.Nil(err)
	require.Equal("\"20000.00000000\"", string(j))
	err = c.UnmarshalJSON(j)
	require.Nil(err)
	require.Equal("20000.00000000", c.String())

	require.Equal(0, b.Add(a).Cmp(c))
	require.Equal(0, c.Sub(a).Cmp(b))
	require.Equal(0, c.Sub(b).Cmp(a))

	a = NewIntegerFromString("0.000000001")
	require.Equal("0.00000000", a.String())
	a = NewIntegerFromString("10.000000001")
	require.Equal("10.00000000", a.String())
	a = NewIntegerFromString("0.00000001")
	require.Equal("0.00000001", a.String())
	a = NewIntegerFromString("10.00000001")
	require.Equal("10.00000001", a.String())
	a = NewIntegerFromString("0.1")
	require.Equal("0.10000000", a.String())

	m := NewInteger(500000)
	n := m.Div(10)
	require.Equal("50000.00000000", n.String())
	n = m.Div(1000000)
	require.Equal("0.50000000", n.String())
	n = n.Div(10000000)
	require.Equal("0.00000005", n.String())
	require.Equal(1, n.Sign())
	n = n.Mul(10).Div(10)
	require.Equal("0.00000005", n.String())
	require.Equal(1, n.Sign())
	n = n.Div(10).Mul(10)
	require.Equal("0.00000000", n.String())
	require.Equal(0, n.Sign())

	m = NewInteger(1)
	n = m.Div(3)
	require.Equal("0.33333333", n.String())
	n = n.Mul(3)
	require.Equal("0.99999999", n.String())
	n = n.Add(NewIntegerFromString("0.00000001"))
	require.Equal("1.00000000", n.String())

	m = NewInteger(8273)
	require.Equal("8273.00000000", m.String())

	m = NewIntegerFromString("0.00000192")
	require.Equal("0.00000192", m.String())

	i0 := NewIntegerFromString("0.003790547948714634")
	require.Equal("0.00379054", i0.String())
	i1 := NewIntegerFromString(i0.String())
	require.Equal("0.00379054", i1.String())
	require.Equal(i0, i1)
}
