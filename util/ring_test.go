package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRingBuffer(t *testing.T) {
	assert := assert.New(t)

	pool := NewRingBuffer(256)
	for i := 0; i < 256; i++ {
		ok, err := pool.Offer(i)
		assert.True(ok)
		assert.Nil(err)
	}
	for i := 0; i < 256; i++ {
		ok, err := pool.Offer(i)
		assert.False(ok)
		assert.Nil(err)
	}
	for i := 0; i < 256; i++ {
		m, err := pool.Poll(false)
		assert.NotNil(m)
		assert.Nil(err)
	}
	for i := 0; i < 256; i++ {
		m, err := pool.Poll(false)
		assert.Nil(m)
		assert.Nil(err)
	}
}
