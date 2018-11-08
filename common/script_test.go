package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScript(t *testing.T) {
	assert := assert.New(t)

	s := Script([]byte{})
	err := s.Validate(0)
	assert.NotNil(err)

	s = Script([]byte{OperatorSum, OperatorSum, 0})
	err = s.Validate(0)
	assert.NotNil(err)

	s = Script([]byte{OperatorCmp, OperatorCmp, 0})
	err = s.Validate(0)
	assert.NotNil(err)

	s = Script([]byte{OperatorCmp, OperatorSum, 0})
	err = s.Validate(0)
	assert.Nil(err)
	s = Script([]byte{OperatorCmp, OperatorSum, 0})
	err = s.Validate(1)
	assert.Nil(err)
	s = Script([]byte{OperatorCmp, OperatorSum, 1})
	err = s.Validate(0)
	assert.NotNil(err)
	s = Script([]byte{OperatorCmp, OperatorSum, 1})
	err = s.Validate(1)
	assert.Nil(err)
	s = Script([]byte{OperatorCmp, OperatorSum, 1})
	err = s.Validate(2)
	assert.Nil(err)

	j, err := s.MarshalJSON()
	assert.Nil(err)
	assert.Equal("\"fffe01\"", string(j))
	err = s.UnmarshalJSON(j)
	assert.Nil(err)
	err = s.Validate(1)
	assert.Nil(err)
	assert.Equal("fffe01", s.String())
}
