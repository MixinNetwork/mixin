package crypto

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCosi(t *testing.T) {
	assert := assert.New(t)

	keys := make([]Key, 31)
	publics := make([]Key, len(keys))
	for i := 0; i < len(keys); i++ {
		seed := NewHash([]byte(fmt.Sprintf("%d", i)))
		keys[i] = NewKeyFromSeed(append(seed[:], seed[:]...))
		publics[i] = keys[i].Public()
	}

	message := []byte("Schnorr Signature in Mixin Kernel")
	randoms := make([]Key, len(keys)*2/3+1)
	masks := make([]int, 0)
	for i := 0; i < len(randoms); i++ {
		r := CosiCommit(&keys[i], message)
		randoms[i] = r.Public()
		masks = append(masks, i)
	}

	cosi, err := CosiAggregateCommitment(randoms, masks)
	assert.Nil(err)
	assert.Equal("429edaddad04026cc2e735c5fd9269382d1580ad5c972c0a5c05dd9f9d7b3f84000000000000000000000000000000000000000000000000000000000000000000000000001fffff", cosi.String())
	assert.Len(cosi.Keys(), len(masks))

	responses := make([][32]byte, len(randoms))
	for i := 0; i < len(responses); i++ {
		s, err := cosi.Response(&keys[i], publics, message)
		assert.Nil(err)
		responses[i] = s
	}

	err = cosi.AggregateResponse(publics, responses, message)
	assert.Nil(err)
	assert.Equal("429edaddad04026cc2e735c5fd9269382d1580ad5c972c0a5c05dd9f9d7b3f84ddc4e9737efd5e771af76878f39916c0bd492c41e57101e9f1c73b0ed3110f0b00000000001fffff", cosi.String())

	A, err := cosi.AggregatePublicKey(publics)
	assert.Nil(err)
	assert.Equal("5ca50e13ae2a966bb810d49892f7ebd4ba8bf03957478e0ae0221b0d1fd7da55", A.String())
	valid := A.Verify(message, cosi.Signature)
	assert.True(valid)
}
