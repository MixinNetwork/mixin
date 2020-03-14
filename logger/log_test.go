package logger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLogger(t *testing.T) {
	assert := assert.New(t)

	out := filterOutput("hello from mixin %d", time.Now().UnixNano())
	assert.Contains(out, "mixin")

	err := SetFilter("bitcoin")
	assert.Nil(err)
	out = filterOutput("hello from mixin %d", time.Now().UnixNano())
	assert.NotContains(out, "mixin")
	out = filterOutput("Bitcoin from mixin %d", time.Now().UnixNano())
	assert.NotContains(out, "mixin")
	out = filterOutput("bitcoin from mixin %d", time.Now().UnixNano())
	assert.Contains(out, "mixin")

	err = SetFilter("(?i)bitcoin")
	assert.Nil(err)
	out = filterOutput("hello from mixin %d", time.Now().UnixNano())
	assert.NotContains(out, "mixin")
	out = filterOutput("Bitcoin from mixin %d", time.Now().UnixNano())
	assert.Contains(out, "mixin")
	out = filterOutput("bitcoin from mixin %d", time.Now().UnixNano())
	assert.Contains(out, "mixin")
	out = filterOutput("ethereum from mixin %d", time.Now().UnixNano())
	assert.NotContains(out, "mixin")

	err = SetFilter("(?i)bitcoin|Mixin")
	assert.Nil(err)
	out = filterOutput("hello from mixin %d", time.Now().UnixNano())
	assert.Contains(out, "mixin")
	out = filterOutput("Bitcoin from mixin %d", time.Now().UnixNano())
	assert.Contains(out, "mixin")
	out = filterOutput("bitcoin from mixin %d", time.Now().UnixNano())
	assert.Contains(out, "mixin")
	out = filterOutput("ethereum from mixin %d", time.Now().UnixNano())
	assert.Contains(out, "mixin")
	out = filterOutput("ethereum or bitcoin %d", time.Now().UnixNano())
	assert.NotContains(out, "mixin")

	la := limiterAvailable("hello from mixin")
	assert.True(la)
	SetLimiter(10)
	for i := 0; i < 10; i++ {
		la := limiterAvailable("hello from mixin")
		assert.True(la)
	}
	la = limiterAvailable("hello from mixin")
	assert.False(la)
	la = limiterAvailable("hello from mixin again")
	assert.True(la)
}
