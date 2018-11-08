package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInteger(t *testing.T) {
	assert := assert.New(t)

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

	assert.Equal(0, b.Add(a).Cmp(c))
	assert.Equal(0, c.Sub(a).Cmp(b))
	assert.Equal(0, c.Sub(b).Cmp(a))
}
