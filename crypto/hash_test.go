package crypto

import (
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
