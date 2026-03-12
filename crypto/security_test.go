package crypto

import (
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

func TestAggregateVerifyRejectsDuplicateSigners(t *testing.T) {
	require := require.New(t)

	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	key := NewKeyFromSeed(seed)
	pub := key.Public()
	msg := Blake3Hash([]byte("duplicate signers"))

	x, err := edwards25519.NewScalar().SetCanonicalBytes(key[:])
	require.Nil(err)
	doubled := edwards25519.NewScalar().Add(x, x)
	var doubledKey Key
	copy(doubledKey[:], doubled.Bytes())
	sig := doubledKey.Sign(msg)

	err = AggregateVerify(&sig, []*Key{&pub, &pub}, []int{0, 0}, msg)
	require.ErrorContains(err, "invalid aggregation signer order")
}

func TestLowOrderKeysAreRejected(t *testing.T) {
	require := require.New(t)

	var identity Key
	identity[0] = 1
	require.False(identity.CheckKey())

	var zero Key
	require.False(zero.CheckKey())

	msg := Blake3Hash([]byte("identity"))
	var sig Signature
	copy(sig[:32], identity[:])
	require.False(identity.Verify(msg, sig))
	require.False(BatchVerify(msg, []*Key{&identity}, []*Signature{&sig}))
}

func TestGhostKeyDerivationRejectsLowOrderMask(t *testing.T) {
	require := require.New(t)

	a := randomKey()
	b := randomKey()
	B := b.Public()
	var identity Key
	identity[0] = 1

	require.Panics(func() {
		_ = DeriveGhostPrivateKey(&identity, &a, &b, 0)
	})
	require.Panics(func() {
		_ = ViewGhostOutputKey(&B, &a, &identity, 0)
	})
}
