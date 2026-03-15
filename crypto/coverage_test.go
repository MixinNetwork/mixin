package crypto

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

func TestHashHelpers(t *testing.T) {
	require := require.New(t)

	var zero Hash
	require.False(zero.HasValue())

	msg := Blake3Hash([]byte("hash helpers"))
	net := Blake3Hash([]byte("testnet"))
	require.True(msg.HasValue())
	require.Equal(Blake3Hash(append(net[:], msg[:]...)), msg.ForNetwork(net))
	require.NotEqual(msg.ForNetwork(net), msg.ForNetwork(Blake3Hash([]byte("mainnet"))))
}

func TestKeyHelpersAndAggregateVerify(t *testing.T) {
	require := require.New(t)

	key := NewKeyFromSeed(testSeed(1))
	parsed, err := KeyFromString(key.String())
	require.Nil(err)
	require.Equal(key, parsed)

	_, err = KeyFromString(key.String()[:len(key.String())-2])
	require.ErrorContains(err, "invalid key size")

	var zero Key
	require.False(zero.HasValue())
	require.True(key.HasValue())

	derived := key.DeterministicHashDerive()
	require.True(derived.HasValue())
	require.Equal(derived, key.DeterministicHashDerive())
	require.NotEqual(key, derived)

	a := NewKeyFromSeed(testSeed(2))
	A := a.Public()
	b := NewKeyFromSeed(testSeed(3))
	B := b.Public()
	r := NewKeyFromSeed(testSeed(4))
	require.Equal(*DeriveGhostPublicKey(&r, &A, &B, 7), *DeriveGhostPublicKeyForInternalVanish(&r, &A, &B, 7))

	k1 := NewKeyFromSeed(testSeed(10))
	k2 := NewKeyFromSeed(testSeed(11))
	k3 := NewKeyFromSeed(testSeed(12))
	p1 := k1.Public()
	p2 := k2.Public()
	p3 := k3.Public()
	publics := []*Key{&p1, &p2, &p3}
	signers := []int{0, 2}
	msg := Blake3Hash([]byte("aggregate verify"))

	aggPriv := aggregatePrivateKeys(k1, k3)
	sig := aggPriv.Sign(msg)
	err = AggregateVerify(&sig, publics, signers, msg)
	require.Nil(err)

	err = AggregateVerify(&sig, publics, []int{3}, msg)
	require.ErrorContains(err, "invalid aggregation signer index")

	sig[0] ^= 0xff
	err = AggregateVerify(&sig, publics, signers, msg)
	require.ErrorContains(err, "signature verify failed")
}

func TestSignaturePartsAndCosiJSON(t *testing.T) {
	require := require.New(t)

	key := NewKeyFromSeed(testSeed(21))
	msg := Blake3Hash([]byte("signature parts"))
	sig := key.Sign(msg)
	require.Equal(sig[:32], sig.R())
	require.Equal(sig[32:], sig.S())

	cosi := CosiSignature{Signature: sig, Mask: 0x123}
	data, err := cosi.MarshalJSON()
	require.Nil(err)

	var decoded CosiSignature
	err = decoded.UnmarshalJSON(data)
	require.Nil(err)
	require.Equal(cosi.Signature, decoded.Signature)
	require.Equal(cosi.Mask, decoded.Mask)

	err = decoded.UnmarshalJSON([]byte(`"abcd"`))
	require.ErrorContains(err, "invalid signature length")
}

func TestCosiCommitAndAggregateErrors(t *testing.T) {
	require := require.New(t)

	require.Panics(func() {
		CosiCommit(shortReader{})
	})

	require.Panics(func() {
		CosiCommit(errReader{})
	})

	bad := Key{1}
	_, err := CosiAggregateCommitment(map[int]*Key{0: &bad})
	require.NotNil(err)

	good := NewKeyFromSeed(testSeed(30)).Public()
	_, err = CosiAggregateCommitment(map[int]*Key{64: &good})
	require.ErrorContains(err, "invalid cosi signature mask index 64")
}

func TestCosiStrictAggregationAndVerificationErrors(t *testing.T) {
	require := require.New(t)

	keys, publics, randoms, cosi, message := buildCosiFixture(require)
	resp0, err := cosi.Response(keys[0], randoms[0], publics, message)
	require.Nil(err)
	resp1, err := cosi.Response(keys[1], randoms[1], publics, message)
	require.Nil(err)

	err = cosi.AggregateResponse(publics, map[int]*[32]byte{0: resp0}, message, false)
	require.ErrorContains(err, "missing key 1")

	err = cosi.AggregateResponse(publics, map[int]*[32]byte{0: resp0, 1: resp1, 2: resp0}, message, false)
	require.ErrorContains(err, "responses count 2/3")

	badResp0, err := cosi.Response(keys[1], randoms[0], publics, message)
	require.Nil(err)
	err = cosi.VerifyResponse(publics, 0, badResp0, message)
	require.ErrorContains(err, "invalid cosi signature response")

	err = cosi.VerifyResponse(publics, 3, resp0, message)
	require.ErrorContains(err, "invalid cosi signature mask index 3")

	err = cosi.AggregateResponse(publics, map[int]*[32]byte{0: badResp0, 1: resp1}, message, true)
	require.ErrorContains(err, "invalid cosi signature response")

	_, _, _, cosi, _ = buildCosiFixture(require)
	resp0, _ = cosi.Response(keys[0], randoms[0], publics, message)
	resp1, _ = cosi.Response(keys[1], randoms[1], publics, message)
	err = cosi.AggregateResponse(publics, map[int]*[32]byte{0: resp0, 1: resp1}, message, true)
	require.Nil(err)

	cosi.Signature[0] ^= 0xff
	err = cosi.FullVerify(publics, 2, message)
	require.ErrorContains(err, "signature verify failed")
}

func TestJSONErrorPathsAndKeyPanics(t *testing.T) {
	require := require.New(t)

	var hash Hash
	err := hash.UnmarshalJSON([]byte("no-quotes"))
	require.NotNil(err)
	err = hash.UnmarshalJSON([]byte(`"zz"`))
	require.NotNil(err)

	var key Key
	err = key.UnmarshalJSON([]byte("no-quotes"))
	require.NotNil(err)
	err = key.UnmarshalJSON([]byte(`"zz"`))
	require.NotNil(err)

	var sig Signature
	err = sig.UnmarshalJSON([]byte("no-quotes"))
	require.NotNil(err)
	err = sig.UnmarshalJSON([]byte(`"zz"`))
	require.NotNil(err)

	require.Panics(func() {
		NewKeyFromSeed(make([]byte, 63))
	})

	var invalid Key
	for i := range invalid {
		invalid[i] = 0xff
	}

	require.Panics(func() {
		invalid.Public()
	})

	validPriv := NewKeyFromSeed(testSeed(50))
	validPub := validPriv.Public()
	require.Panics(func() {
		KeyMultPubPriv(&invalid, &validPriv)
	})
	require.Panics(func() {
		KeyMultPubPriv(&validPub, &invalid)
	})
	require.Panics(func() {
		DeriveGhostPublicKey(&validPriv, &validPub, &invalid, 0)
	})
	require.Panics(func() {
		DeriveGhostPrivateKey(&validPub, &validPriv, &invalid, 0)
	})
}

func TestReadRandBranches(t *testing.T) {
	require := require.New(t)

	require.Panics(func() {
		ReadRand(nil)
	})

	buf := make([]byte, 3)
	ReadRand(buf)
	require.NotEqual([]byte{0, 0, 0}, buf)
}

func aggregatePrivateKeys(keys ...Key) Key {
	sum := edwards25519.NewScalar()
	for _, key := range keys {
		scalar, err := edwards25519.NewScalar().SetCanonicalBytes(key[:])
		if err != nil {
			panic(err)
		}
		sum = sum.Add(sum, scalar)
	}

	var aggregated Key
	copy(aggregated[:], sum.Bytes())
	return aggregated
}

func testSeed(base byte) []byte {
	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = base + byte(i)
	}
	return seed
}

func buildCosiFixture(require *require.Assertions) ([]*Key, []*Key, []*Key, *CosiSignature, Hash) {
	priv0 := NewKeyFromSeed(testSeed(40))
	priv1 := NewKeyFromSeed(testSeed(41))
	pub0 := priv0.Public()
	pub1 := priv1.Public()

	rand0 := NewKeyFromSeed(testSeed(42))
	rand1 := NewKeyFromSeed(testSeed(43))
	commit0 := rand0.Public()
	commit1 := rand1.Public()

	cosi, err := CosiAggregateCommitment(map[int]*Key{0: &commit0, 1: &commit1})
	require.Nil(err)

	keys := []*Key{&priv0, &priv1}
	publics := []*Key{&pub0, &pub1}
	randoms := []*Key{&rand0, &rand1}
	return keys, publics, randoms, cosi, Blake3Hash([]byte("cosi coverage"))
}

type shortReader struct{}

func (shortReader) Read(b []byte) (int, error) {
	return len(b) - 1, nil
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read failure")
}

func TestPanicMessagesDoNotLeakKeyMaterial(t *testing.T) {
	require := require.New(t)

	// Create an invalid key (all 0xff bytes are not a valid scalar)
	var invalid Key
	for i := range invalid {
		invalid[i] = 0xff
	}
	invalidHex := invalid.String()

	// Create valid keys for testing
	validPriv := NewKeyFromSeed(testSeed(60))
	validPub := validPriv.Public()
	validPrivHex := validPriv.String()

	// Helper to capture panic message and verify it doesn't contain key hex
	assertPanicDoesNotContainKey := func(fn func(), forbiddenStrings ...string) {
		var panicMsg string
		func() {
			defer func() {
				r := recover()
				require.NotNil(r)
				panicMsg = fmt.Sprintf("%v", r)
			}()
			fn()
		}()
		for _, forbidden := range forbiddenStrings {
			require.False(strings.Contains(panicMsg, forbidden),
				"panic message should not contain key material: %s", panicMsg)
		}
	}

	// Public() should not leak private key bytes
	assertPanicDoesNotContainKey(func() {
		invalid.Public()
	}, invalidHex)

	// KeyMultPubPriv should not leak public or private key bytes
	assertPanicDoesNotContainKey(func() {
		KeyMultPubPriv(&invalid, &validPriv)
	}, invalidHex, validPrivHex)
	assertPanicDoesNotContainKey(func() {
		KeyMultPubPriv(&validPub, &invalid)
	}, invalidHex, validPrivHex)

	// DeriveGhostPublicKey should not leak key bytes
	assertPanicDoesNotContainKey(func() {
		DeriveGhostPublicKey(&validPriv, &validPub, &invalid, 0)
	}, invalidHex)

	// DeriveGhostPrivateKey should not leak private key bytes
	assertPanicDoesNotContainKey(func() {
		DeriveGhostPrivateKey(&validPub, &validPriv, &invalid, 0)
	}, invalidHex, validPrivHex)

	// Sign should not leak private key (need a valid private key that fails SetCanonicalBytes)
	// This is hard to trigger since Sign() creates a valid scalar from a valid private key.
	// Instead, verify the cosi.Response path
	keys, publics, randoms, cosi, message := buildCosiFixture(require)
	_ = keys
	_ = randoms

	assertPanicDoesNotContainKey(func() {
		cosi.Response(&invalid, randoms[0], publics, message)
	}, invalidHex)
	assertPanicDoesNotContainKey(func() {
		cosi.Response(keys[0], &invalid, publics, message)
	}, invalidHex)
}
