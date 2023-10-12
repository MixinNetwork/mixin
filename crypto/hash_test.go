package crypto

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHash(t *testing.T) {
	require := require.New(t)

	seed := make([]byte, 64)
	for i := 0; i < len(seed); i++ {
		seed[i] = byte(i + 1)
	}

	h := Sha256Hash(seed)
	require.Equal("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358", h.String())
	h, err := HashFromString("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358")
	require.Nil(err)
	require.Equal("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358", h.String())

	j, err := h.MarshalJSON()
	require.Nil(err)
	require.Equal("\"9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358\"", string(j))
	err = h.UnmarshalJSON(j)
	require.Nil(err)
	require.Equal("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70358", h.String())

	h, err = HashFromString("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70357")
	require.Nil(err)
	require.Equal("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb70357", h.String())
	h, err = HashFromString("9323516a9ed2b789339472e38673fd74e8e802efbb94b0b9454f0188ccb7035")
	require.NotNil(err)
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
					Sha256Hash([]byte(msg))
				} else {
					Blake3Hash([]byte(msg))
				}
			}
		})
	}
}
