package crypto

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHash(t *testing.T) {
	assert := assert.New(t)

	seed := make([]byte, 64)
	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 1)
	}

	h := NewHash(seed)
	assert.Equal("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358", h.String())
	h, err := HashFromString("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358")
	assert.Nil(err)
	assert.Equal("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358", h.String())

	j, err := h.MarshalJSON()
	assert.Nil(err)
	assert.Equal("\"9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358\"", string(j))
	err = h.UnmarshalJSON(j)
	assert.Nil(err)
	assert.Equal("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358", h.String())

	h, err = HashFromString("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70357")
	assert.Nil(err)
	assert.Equal("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70357", h.String())
	h, err = HashFromString("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb7035")
	assert.NotNil(err)
}

func BenchmarkHash(b *testing.B) {
	benchmarkHash(b, false)
}

func BenchmarkHashLegacy(b *testing.B) {
	benchmarkHash(b, true)
}

func benchmarkHash(b *testing.B, legacy bool) {
	msg := "We build open source software that always puts security, privacy and decentralization first."
	for _, n := range []int{1, 2, 4, 8} {
		for i := 0; i < n; i++ {
			msg = msg + msg
		}
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if legacy {
					NewHash([]byte(msg))
				} else {
					Blake3Hash([]byte(msg))
				}
			}
		})
	}
}
