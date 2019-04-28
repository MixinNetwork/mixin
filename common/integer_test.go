package common

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInteger(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("0.00000000", NewInteger(0).String())

	a := NewInteger(10000)
	b := NewIntegerFromString("10000")
	assert.Equal(0, a.Cmp(b))

	c := a.Add(b)
	assert.Equal("20000.00000000", c.String())
	j, err := c.MarshalJSON()
	assert.Nil(err)
	assert.Equal("\"20000.00000000\"", string(j))
	err = c.UnmarshalJSON(j)
	assert.Nil(err)
	assert.Equal("20000.00000000", c.String())
	p, err := c.MarshalMsgpack()
	assert.Nil(err)
	assert.Equal("01d1a94a2000", hex.EncodeToString(p))
	err = c.UnmarshalMsgpack(p)
	assert.Nil(err)
	assert.Equal("20000.00000000", c.String())

	assert.Equal(0, b.Add(a).Cmp(c))
	assert.Equal(0, c.Sub(a).Cmp(b))
	assert.Equal(0, c.Sub(b).Cmp(a))

	a = NewIntegerFromString("0.000000001")
	assert.Equal("0.00000000", a.String())
	a = NewIntegerFromString("10.000000001")
	assert.Equal("10.00000000", a.String())
	a = NewIntegerFromString("0.00000001")
	assert.Equal("0.00000001", a.String())
	a = NewIntegerFromString("10.00000001")
	assert.Equal("10.00000001", a.String())
	a = NewIntegerFromString("0.1")
	assert.Equal("0.10000000", a.String())

	m := NewInteger(500000)
	n := m.Div(10)
	assert.Equal("50000.00000000", n.String())
	n = m.Div(1000000)
	assert.Equal("0.50000000", n.String())
	n = n.Div(10000000)
	assert.Equal("0.00000005", n.String())
	assert.Equal(1, n.Sign())
	n = n.Mul(10).Div(10)
	assert.Equal("0.00000005", n.String())
	assert.Equal(1, n.Sign())
	n = n.Div(10).Mul(10)
	assert.Equal("0.00000000", n.String())
	assert.Equal(0, n.Sign())

	m = NewInteger(1)
	n = m.Div(3)
	assert.Equal("0.33333333", n.String())
	n = n.Mul(3)
	assert.Equal("0.99999999", n.String())
	n = n.Add(NewIntegerFromString("0.00000001"))
	assert.Equal("1.00000000", n.String())

	m = NewInteger(8273)
	assert.Equal("8273.00000000", m.String())
	p = MsgpackMarshalPanic(m)
	assert.Equal("c70500c09eec3100", hex.EncodeToString(p))
	err = MsgpackUnmarshal(p, &m)
	assert.Nil(err)
	assert.Equal("8273.00000000", m.String())

	m = NewIntegerFromString("0.00000192")
	assert.Equal("0.00000192", m.String())
	p = MsgpackMarshalPanic(m)
	assert.Equal("d400c0", hex.EncodeToString(p))
	err = MsgpackUnmarshal(p, &m)
	assert.Nil(err)
	assert.Equal("0.00000192", m.String())
}
