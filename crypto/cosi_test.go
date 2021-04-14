package crypto

import (
	"crypto/rand"
	"fmt"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/assert"
	"go.dedis.ch/kyber/v3/xof/blake2xb"
)

func TestCosi(t *testing.T) {
	assert := assert.New(t)

	assert.NotEqual(CosiCommit(rand.Reader).String(), CosiCommit(rand.Reader).String())

	keys := make([]*Key, 31)
	publics := make([]*Key, len(keys))
	for i := 0; i < len(keys); i++ {
		seed := NewHash([]byte(fmt.Sprintf("%d", i)))
		priv := NewKeyFromSeed(append(seed[:], seed[:]...))
		pub := priv.Public()
		keys[i] = &priv
		publics[i] = &pub
	}

	P := edwards25519.NewIdentityPoint()
	for i, k := range publics {
		if i >= len(publics)*2/3+1 {
			break
		}
		p, err := edwards25519.NewIdentityPoint().SetBytes(k[:])
		assert.Nil(err)
		P = P.Add(P, p)
	}
	var aggregatedPublic Key
	copy(aggregatedPublic[:], P.Bytes())
	assert.Equal("5ca50e13ae2a966bb810d49892f7ebd4ba8bf03957478e0ae0221b0d1fd7da55", aggregatedPublic.String())

	randReader := blake2xb.New(nil)
	message := []byte("Schnorr Signature in Mixin Kernel")
	randoms := make(map[int]*Key)
	randKeys := make([]*Key, len(keys)*2/3+1)
	masks := make([]int, 0)
	for i := 0; i < 7; i++ {
		r := CosiCommit(randReader)
		R := r.Public()
		randKeys[i] = r
		randoms[i] = &R
		masks = append(masks, i)
	}
	for i := 10; i < len(randKeys)+3; i++ {
		r := CosiCommit(randReader)
		R := r.Public()
		randKeys[i-3] = r
		randoms[i] = &R
		masks = append(masks, i)
	}
	assert.Len(masks, len(randoms))

	cosi, err := CosiAggregateCommitment(randoms)
	assert.Nil(err)
	assert.Equal("81a085ca768adc4901b5484ecc3cdbb4eee68307f78cd5ea041d7d4425496bd100000000000000000000000000000000000000000000000000000000000000000000000000fffc7f", cosi.String())
	assert.Equal(masks, cosi.Keys())

	responses := make(map[int]*[32]byte)
	for i := 0; i < len(masks); i++ {
		s, err := cosi.Response(keys[masks[i]], randKeys[i], publics, message)
		assert.Nil(err)
		responses[masks[i]] = s
		assert.Equal("81a085ca768adc4901b5484ecc3cdbb4eee68307f78cd5ea041d7d4425496bd100000000000000000000000000000000000000000000000000000000000000000000000000fffc7f", cosi.String())
		err = cosi.VerifyResponse(publics, masks[i], s, message)
		assert.Nil(err)
	}

	err = cosi.AggregateResponse(publics, responses, message, true)
	assert.Nil(err)
	assert.Equal("81a085ca768adc4901b5484ecc3cdbb4eee68307f78cd5ea041d7d4425496bd142d036ee5382af36ba979ddbaaf7023f5e59cb79d884642a7b1cf662adedb7040000000000fffc7f", cosi.String())

	A, err := cosi.aggregatePublicKey(publics)
	assert.Nil(err)
	assert.Equal("b5b493bbce28209e2c24030db057554ee3d683235011ccfb21b7e615c74d937f", A.String())
	valid := A.Verify(message, cosi.Signature)
	assert.True(valid)

	valid = cosi.ThresholdVerify(len(randoms) + 1)
	assert.False(valid)
	valid = cosi.ThresholdVerify(len(randoms))
	assert.True(valid)
	err = cosi.FullVerify(publics, len(randoms)+1, message)
	assert.NotNil(err)
	err = cosi.FullVerify(publics, len(randoms), message)
	assert.Nil(err)
}
