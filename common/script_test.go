package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScript(t *testing.T) {
	require := require.New(t)

	s := Script([]byte{})
	err := s.Validate(0)
	require.NotNil(err)

	s = Script([]byte{OperatorSum, OperatorSum, 0})
	err = s.Validate(0)
	require.NotNil(err)

	s = Script([]byte{OperatorCmp, OperatorCmp, 0})
	err = s.Validate(0)
	require.NotNil(err)

	s = Script([]byte{OperatorCmp, OperatorSum, 0})
	err = s.Validate(0)
	require.Nil(err)
	s = Script([]byte{OperatorCmp, OperatorSum, 0})
	err = s.Validate(1)
	require.Nil(err)
	s = Script([]byte{OperatorCmp, OperatorSum, 1})
	err = s.Validate(0)
	require.NotNil(err)
	s = Script([]byte{OperatorCmp, OperatorSum, 1})
	err = s.Validate(1)
	require.Nil(err)
	s = Script([]byte{OperatorCmp, OperatorSum, 1})
	err = s.Validate(2)
	require.Nil(err)

	j, err := s.MarshalJSON()
	require.Nil(err)
	require.Equal("\"fffe01\"", string(j))
	err = s.UnmarshalJSON(j)
	require.Nil(err)
	err = s.Validate(1)
	require.Nil(err)
	require.Equal("fffe01", s.String())
}
