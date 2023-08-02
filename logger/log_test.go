package logger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	require := require.New(t)

	out := filterOutput("hello from mixin %d", time.Now().UnixNano())
	require.Contains(out, "mixin")

	err := SetFilter("bitcoin")
	require.Nil(err)
	out = filterOutput("hello from mixin %d", time.Now().UnixNano())
	require.NotContains(out, "mixin")
	out = filterOutput("Bitcoin from mixin %d", time.Now().UnixNano())
	require.NotContains(out, "mixin")
	out = filterOutput("bitcoin from mixin %d", time.Now().UnixNano())
	require.Contains(out, "mixin")

	err = SetFilter("(?i)bitcoin")
	require.Nil(err)
	out = filterOutput("hello from mixin %d", time.Now().UnixNano())
	require.NotContains(out, "mixin")
	out = filterOutput("Bitcoin from mixin %d", time.Now().UnixNano())
	require.Contains(out, "mixin")
	out = filterOutput("bitcoin from mixin %d", time.Now().UnixNano())
	require.Contains(out, "mixin")
	out = filterOutput("ethereum from mixin %d", time.Now().UnixNano())
	require.NotContains(out, "mixin")

	err = SetFilter("(?i)bitcoin|Mixin")
	require.Nil(err)
	out = filterOutput("hello from mixin %d", time.Now().UnixNano())
	require.Contains(out, "mixin")
	out = filterOutput("Bitcoin from mixin %d", time.Now().UnixNano())
	require.Contains(out, "mixin")
	out = filterOutput("bitcoin from mixin %d", time.Now().UnixNano())
	require.Contains(out, "mixin")
	out = filterOutput("ethereum from mixin %d", time.Now().UnixNano())
	require.Contains(out, "mixin")
	out = filterOutput("ethereum or bitcoin %d", time.Now().UnixNano())
	require.NotContains(out, "mixin")

	level = 0
	filter = nil
}
