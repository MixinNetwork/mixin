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

func TestAggregateVerifyRejectsRogueKeyForgery(t *testing.T) {
	require := require.New(t)

	victimPriv := NewKeyFromSeed(testSeed(60))
	victimPub := victimPriv.Public()

	attackerPriv := NewKeyFromSeed(testSeed(61))
	attackerPub := attackerPriv.Public()

	victimPoint, err := edwards25519.NewIdentityPoint().SetBytes(victimPub[:])
	require.Nil(err)
	attackerPoint, err := edwards25519.NewIdentityPoint().SetBytes(attackerPub[:])
	require.Nil(err)

	roguePoint := edwards25519.NewIdentityPoint().Subtract(attackerPoint, victimPoint)
	var roguePub Key
	copy(roguePub[:], roguePoint.Bytes())
	require.True(roguePub.CheckKey())

	msg := Blake3Hash([]byte("rogue aggregate verify"))
	sig := attackerPriv.Sign(msg)

	err = AggregateVerify(&sig, []*Key{&victimPub, &roguePub}, []int{0, 1}, msg)
	require.ErrorContains(err, "signature verify failed")
}

func TestAggregateSignSingleSigner(t *testing.T) {
	require := require.New(t)

	priv := NewKeyFromSeed(testSeed(70))
	pub := priv.Public()
	publics := []*Key{&pub}
	signers := []int{0}
	msg := Blake3Hash([]byte("single signer aggregate"))

	sig, err := AggregateSign([]*Key{&priv}, publics, signers, testSeed(71), msg)
	require.Nil(err)
	err = AggregateVerify(sig, publics, signers, msg)
	require.Nil(err)

	wrong := Blake3Hash([]byte("wrong message"))
	err = AggregateVerify(sig, publics, signers, wrong)
	require.ErrorContains(err, "signature verify failed")
}

func TestAggregateSignRejectsKeyCountMismatch(t *testing.T) {
	require := require.New(t)

	k1 := NewKeyFromSeed(testSeed(72))
	k2 := NewKeyFromSeed(testSeed(73))
	p1 := k1.Public()
	p2 := k2.Public()
	msg := Blake3Hash([]byte("key count mismatch"))

	_, err := AggregateSign([]*Key{&k1}, []*Key{&p1, &p2}, []int{0, 1}, testSeed(74), msg)
	require.ErrorContains(err, "invalid aggregation private keys count")
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
