package ed25519

import (
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestCosi(t *testing.T) {
	assert := assert.New(t)

	var (
		raw = []byte("just a test")

		privates    = make(map[int]crypto.PrivateKey, 20)
		randoms     = make(map[int]crypto.PrivateKey, 20)
		publics     = make(map[int]crypto.PublicKey, 20)
		commitments = make(map[int]*crypto.Commitment, 20)

		aggPrivate crypto.PrivateKey
		aggPublic  crypto.PublicKey
		aggRand    crypto.PublicKey
	)

	for i := 0; i < 20; i++ {
		var (
			p          = randomKey()
			r          = randomKey()
			P          = p.Public()
			R          = r.Public()
			commitment = crypto.Commitment(R.Key())
		)
		privates[i] = p
		publics[i] = P

		randoms[i] = r
		commitments[i] = &commitment

		if i == 0 {
			aggPrivate = p
			aggPublic = P
			aggRand = R
		} else {
			aggPrivate = aggPrivate.AddPrivate(p)
			aggPublic = aggPublic.AddPublic(P)
			aggRand = aggRand.AddPublic(R)
		}
	}

	assert.Equal(aggPrivate.Public().String(), aggPublic.String())

	cosi, err := crypto.CosiAggregateCommitments(commitments)
	assert.Nil(err)
	if !assert.Equal(1, len(cosi.Signatures)) {
		panic("failed")
	}

	pubs := make([]crypto.PublicKey, 0, len(publics))
	for _, P := range publics {
		pubs = append(pubs, P)
	}
	hReduced, err := cosi.Challenge(pubs, raw)
	assert.Nil(err)
	assert.Equal(hReduced, aggPublic.Challenge(aggRand, raw))

	{
		for i, p := range privates {
			sig := p.SignWithChallenge(randoms[i], raw, hReduced)
			assert.True(publics[i].VerifyWithChallenge(raw, sig, hReduced))
			assert.Nil(cosi.AggregateSignature(i, sig))
		}

		assert.Equal(1, len(cosi.Signatures))
		assert.True(aggPublic.Verify(raw, &cosi.Signatures[0]))
		assert.True(cosi.FullVerify(pubs, len(pubs), raw))
	}
}
