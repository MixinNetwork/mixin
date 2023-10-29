package crypto

import (
	"fmt"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

func TestCosi(t *testing.T) {
	require := require.New(t)

	require.NotEqual(CosiCommit(RandReader()).String(), CosiCommit(RandReader()).String())

	keys := make([]*Key, 31)
	publics := make([]*Key, len(keys))
	for i := 0; i < len(keys); i++ {
		seed := Blake3Hash([]byte(fmt.Sprintf("%d", i)))
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
		require.Nil(err)
		P = P.Add(P, p)
	}
	var aggregatedPublic Key
	copy(aggregatedPublic[:], P.Bytes())
	require.Equal("f77fde77d032e4f828f06aaa4c92c7690fabda792eebe10a7ce57313da8a3b50", aggregatedPublic.String())

	randReader := NewBlake2bXOF(nil)
	message := Blake3Hash([]byte("Schnorr Signature in Mixin Kernel"))
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
	require.Len(masks, len(randoms))

	cosi, err := CosiAggregateCommitment(randoms)
	require.Nil(err)
	require.Equal("81a085ca768adc4901b5484ecc3cdbb4eee68307f78cd5ea041d7d4425496bd100000000000000000000000000000000000000000000000000000000000000000000000000fffc7f", cosi.String())
	require.Equal(masks, cosi.Keys())

	responses := make(map[int]*[32]byte)
	for i := 0; i < len(masks); i++ {
		s, err := cosi.Response(keys[masks[i]], randKeys[i], publics, message)
		require.Nil(err)
		responses[masks[i]] = s
		require.Equal("81a085ca768adc4901b5484ecc3cdbb4eee68307f78cd5ea041d7d4425496bd100000000000000000000000000000000000000000000000000000000000000000000000000fffc7f", cosi.String())
		err = cosi.VerifyResponse(publics, masks[i], s, message)
		require.Nil(err)
	}

	err = cosi.AggregateResponse(publics, responses, message, true)
	require.Nil(err)
	require.Equal("81a085ca768adc4901b5484ecc3cdbb4eee68307f78cd5ea041d7d4425496bd1cc3db3a447f24fb271dcc0d2de2ee5828e937e853a20a9f199924c697cf1d3040000000000fffc7f", cosi.String())

	A, err := cosi.aggregatePublicKey(publics)
	require.Nil(err)
	require.Equal("6b0a9ff114f5b61e97e62025cc877e78344917a10b2b828b21140f9459df6135", A.String())
	valid := A.Verify(message, cosi.Signature)
	require.True(valid)

	valid = cosi.ThresholdVerify(len(randoms) + 1)
	require.False(valid)
	valid = cosi.ThresholdVerify(len(randoms))
	require.True(valid)
	err = cosi.FullVerify(publics, len(randoms)+1, message)
	require.NotNil(err)
	err = cosi.FullVerify(publics, len(randoms), message)
	require.Nil(err)
}
