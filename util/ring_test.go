package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRingBuffer(t *testing.T) {
	require := require.New(t)

	pool := NewRingBuffer(256)
	for i := 0; i < 256; i++ {
		ok, err := pool.Offer(i)
		require.True(ok)
		require.Nil(err)
	}
	for i := 0; i < 256; i++ {
		ok, err := pool.Offer(i)
		require.False(ok)
		require.Nil(err)
	}
	for i := 0; i < 256; i++ {
		m, err := pool.Poll(false)
		require.NotNil(m)
		require.Nil(err)
	}
	for i := 0; i < 256; i++ {
		m, err := pool.Poll(false)
		require.Nil(m)
		require.Nil(err)
	}
}
