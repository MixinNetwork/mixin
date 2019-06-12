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
	for i := 0; i < 7; i++ {
		r := CosiCommit(keys[i], message)
		R := r.Public()
		randoms[i] = &R
		masks = append(masks, i)
	}
	for i := 10; i < len(randoms)+3; i++ {
		r := CosiCommit(keys[i], message)
		R := r.Public()
		randoms[i-3] = &R
		masks = append(masks, i)
	}
	assert.Len(masks, len(randoms))

	cosi, err := CosiAggregateCommitment(randoms, masks)
	assert.Nil(err)
	assert.Equal("c49dfba9ec603e5e66e71dba3fdb5050af8ea0c4738023b3b90ef0f9188da68a00000000000000000000000000000000000000000000000000000000000000000000000000fffc7f", cosi.String())
	assert.Equal(masks, cosi.Keys())

	responses := make([]*[32]byte, len(randoms))
	for i := 0; i < len(responses); i++ {
		s, err := cosi.Response(keys[masks[i]], publics, message)
		assert.Nil(err)
		responses[i] = &s
		assert.Equal("c49dfba9ec603e5e66e71dba3fdb5050af8ea0c4738023b3b90ef0f9188da68a00000000000000000000000000000000000000000000000000000000000000000000000000fffc7f", cosi.String())
		err = cosi.VerifyResponse(publics, masks[i], &s, message)
		assert.Nil(err)
	}

	err = cosi.AggregateResponse(publics, responses, message)
	assert.Nil(err)
	assert.Equal("c49dfba9ec603e5e66e71dba3fdb5050af8ea0c4738023b3b90ef0f9188da68a4f09994b2e5824b024c9c1165fdaca04e431e9a15862cb52ebb6ddb230eb65010000000000fffc7f", cosi.String())

	A, err := cosi.AggregatePublicKey(publics)
	assert.Nil(err)
	assert.Equal("b5b493bbce28209e2c24030db057554ee3d683235011ccfb21b7e615c74d937f", A.String())
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
