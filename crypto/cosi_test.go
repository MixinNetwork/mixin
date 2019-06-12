package crypto

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCosi(t *testing.T) {
	assert := assert.New(t)

	keys := make([]*Key, 31)
	publics := make([]*Key, len(keys))
	for i := 0; i < len(keys); i++ {
		seed := NewHash([]byte(fmt.Sprintf("%d", i)))
		priv := NewKeyFromSeed(append(seed[:], seed[:]...))
		pub := priv.Public()
		keys[i] = &priv
		publics[i] = &pub
	}

	var aggregatedPublic *Key
	for i, k := range publics {
		if i >= len(publics)*2/3+1 {
			break
		}
		if aggregatedPublic == nil {
			aggregatedPublic = k
		} else {
			aggregatedPublic = KeyAddPub(aggregatedPublic, k)
		}
	}
	assert.Equal("5ca50e13ae2a966bb810d49892f7ebd4ba8bf03957478e0ae0221b0d1fd7da55", aggregatedPublic.String())

	message := []byte("Schnorr Signature in Mixin Kernel")
	randoms := make([]*Key, len(keys)*2/3+1)
	masks := make([]int, 0)
	for i := 0; i < len(randoms); i++ {
		r := CosiCommit(keys[i], message)
		R := r.Public()
		randoms[i] = &R
		masks = append(masks, i)
	}

	cosi, err := CosiAggregateCommitment(randoms, masks)
	assert.Nil(err)
	assert.Equal("fd7285836bea7b418ed66c9d63c04f801633929bff37206dd7d02306f7bc1522000000000000000000000000000000000000000000000000000000000000000000000000001fffff", cosi.String())
	assert.Equal(masks, cosi.Keys())

	responses := make([]*[32]byte, len(randoms))
	for i := 0; i < len(responses); i++ {
		s, err := cosi.Response(keys[masks[i]], publics, message)
		assert.Nil(err)
		responses[i] = &s
		assert.Equal("fd7285836bea7b418ed66c9d63c04f801633929bff37206dd7d02306f7bc1522000000000000000000000000000000000000000000000000000000000000000000000000001fffff", cosi.String())
	}

	err = cosi.AggregateResponse(publics, responses, message)
	assert.Nil(err)
	assert.Equal("fd7285836bea7b418ed66c9d63c04f801633929bff37206dd7d02306f7bc1522955f152daae0de629b83e7770b15bfae3f3f1c9ce18382a914dddd0caa7d3d0700000000001fffff", cosi.String())

	A, err := cosi.AggregatePublicKey(publics)
	assert.Nil(err)
	assert.Equal("5ca50e13ae2a966bb810d49892f7ebd4ba8bf03957478e0ae0221b0d1fd7da55", A.String())
	valid := A.Verify(message, cosi.Signature)
	assert.True(valid)

	valid = cosi.ThresholdVerify(len(randoms) + 1)
	assert.False(valid)
	valid = cosi.ThresholdVerify(len(randoms))
	assert.True(valid)
	valid = cosi.FullVerify(publics, len(randoms)+1, message)
	assert.False(valid)
	valid = cosi.FullVerify(publics, len(randoms), message)
	assert.True(valid)
}
