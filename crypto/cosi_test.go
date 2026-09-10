package crypto

import (
	"bytes"
	"fmt"
	"testing"

	"filippo.io/edwards25519"
	"github.com/stretchr/testify/require"
)

func TestCosiCommitGeneratesDistinctNonces(t *testing.T) {
	first := cosiCommit(RandReader())
	second := cosiCommit(RandReader())
	require.NotEqual(t, *first, *second)
}

func testCosiSigningPair(t *testing.T) ([]*Key, []*Key, Hash) {
	t.Helper()

	priv0 := NewKeyFromSeed(bytes.Repeat([]byte{21}, 64))
	priv1 := NewKeyFromSeed(bytes.Repeat([]byte{22}, 64))
	pub0 := priv0.Public()
	pub1 := priv1.Public()
	return []*Key{&priv0, &priv1}, []*Key{&pub0, &pub1}, Blake3Hash([]byte("cosi pair session"))
}

func TestCosiNonceCoefficientSessionBinding(t *testing.T) {
	require := require.New(t)
	_, publics, message := testCosiSigningPair(t)

	nonce := CosiCommitNonce(RandReader())
	peer1 := CosiCommitNonce(RandReader())
	peer2 := CosiCommitNonce(RandReader())
	first, err := CosiAggregateCommitment(map[int]*CosiCommitment{0: nonce.Public(), 1: peer1.Public()}, publics, message)
	require.Nil(err)
	second, err := CosiAggregateCommitment(map[int]*CosiCommitment{0: nonce.Public(), 1: peer2.Public()}, publics, message)
	require.Nil(err)

	// The nonce coefficient and the effective random depend on the whole
	// session: same commitments for signer 0, different aggregate randoms.
	b1, err := first.nonceCoefficient(publics, message)
	require.Nil(err)
	b2, err := second.nonceCoefficient(publics, message)
	require.Nil(err)
	require.NotEqual(b1.Bytes(), b2.Bytes())
	require.NotEqual(first.Signature[:32], second.Signature[:32])

	// A signature outside the interactive protocol has no coefficient.
	_, err = (&CosiSignature{Mask: 1}).nonceCoefficient(publics, message)
	require.ErrorContains(err, "missing cosi randoms")
	require.NotNil(first.Randoms())
}

func TestCosiWireReconstructionFlow(t *testing.T) {
	require := require.New(t)
	keys, publics, message := testCosiSigningPair(t)

	leaderNonce := CosiCommitNonce(RandReader())
	followerNonce := CosiCommitNonce(RandReader())
	commitments := map[int]*CosiCommitment{0: leaderNonce.Public(), 1: followerNonce.Public()}
	cosi, err := CosiAggregateCommitment(commitments, publics, message)
	require.Nil(err)

	leaderResponse, err := leaderNonce.Response(cosi, keys[0], publics, message)
	require.Nil(err)
	copy(cosi.Signature[32:], leaderResponse[:])

	// The follower reconstructs the session from the wire: mask, the
	// leader's response, and the aggregate randoms, without the effective
	// random.
	var wire CosiSignature
	wire.Mask = cosi.Mask
	copy(wire.Signature[32:], leaderResponse[:])
	require.Nil(wire.Randoms())
	wire.SetRandoms(cosi.Randoms())
	_, err = wire.ensureEffectiveRandom(publics, message)
	require.Nil(err)
	require.Equal(cosi.Signature[:32], wire.Signature[:32])

	// The follower verifies the leader's response against the leader's
	// commitment pair before responding.
	challenge, err := wire.Challenge(publics, message)
	require.Nil(err)
	coefficient, err := wire.nonceCoefficient(publics, message)
	require.Nil(err)
	require.True(verifyCosiPairResponse(publics[0], commitments[0], leaderResponse, coefficient, challenge))

	followerResponse, err := followerNonce.Response(&wire, keys[1], publics, message)
	require.Nil(err)
	require.Nil(cosi.VerifyResponse(publics, 1, followerResponse, message))
	err = cosi.AggregateResponse(publics, map[int]*[32]byte{0: leaderResponse, 1: followerResponse}, message, true)
	require.Nil(err)
	require.Nil(cosi.FullVerify(publics, 2, message))

	// Randoms from another session must not match the embedded effective
	// random.
	stranger := CosiCommitNonce(RandReader())
	mismatched := &CosiSignature{Mask: cosi.Mask, Signature: cosi.Signature}
	mismatched.SetRandoms(&CosiCommitment{rPub1: stranger.Public().rPub1, rPub2: cosi.Randoms().rPub2})
	_, err = mismatched.ensureEffectiveRandom(publics, message)
	require.ErrorContains(err, "effective random mismatch")
}

func TestCosiSigningRejectsInvalidSession(t *testing.T) {
	replacement := testCosiKey(54).Public()
	cases := []struct {
		name   string
		mutate func(*CosiSignature)
		error  string
	}{
		{"effective_random", func(c *CosiSignature) { copy(c.Signature[:32], replacement[:]) }, "effective random mismatch"},
		{"aggregate_R1", func(c *CosiSignature) { c.randoms.rPub1 = replacement }, "effective random mismatch"},
		{"aggregate_R2", func(c *CosiSignature) { c.randoms.rPub2 = replacement }, "effective random mismatch"},
		{"missing_randoms", func(c *CosiSignature) { c.SetRandoms(nil) }, "missing cosi randoms"},
		{"invalid_R1", func(c *CosiSignature) { c.randoms.rPub1 = Key{} }, "invalid point subgroup"},
		{"invalid_R2", func(c *CosiSignature) { c.randoms.rPub2 = Key{} }, "invalid point subgroup"},
		{"identity_R1", func(c *CosiSignature) { c.randoms.rPub1 = Key{1} }, "invalid point subgroup"},
		{"identity_R2", func(c *CosiSignature) { c.randoms.rPub2 = Key{1} }, "invalid point subgroup"},
	}
	for _, api := range []string{"signature", "nonce"} {
		t.Run(api, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					private := testCosiKey(51)
					public := private.Public()
					publics := []*Key{&public}
					random1, random2 := testCosiKey(52), testCosiKey(53)
					nonce := newCosiNonce(&random1, &random2)
					message := Blake3Hash([]byte("validate cosi signing session"))
					signature := testCosiAggregateCommitment(t, map[int]*CosiCommitment{0: nonce.Public()}, publics, message)
					invalid := *signature
					randoms := *signature.Randoms()
					invalid.SetRandoms(&randoms)
					tc.mutate(&invalid)
					respond := func(c *CosiSignature) (*[32]byte, error) {
						if api == "nonce" {
							return nonce.Response(c, &private, publics, message)
						}
						return c.Response(&private, &random1, &random2, publics, message)
					}

					response, err := respond(&invalid)
					require.ErrorContains(t, err, tc.error)
					require.Nil(t, response)

					// Rejection must not consume the nonce or prevent a valid response.
					first, err := respond(signature)
					require.NoError(t, err)
					require.NoError(t, signature.VerifyResponse(publics, 0, first, message))

					// A used nonce must still validate the session before checking its cache.
					response, err = respond(&invalid)
					require.ErrorContains(t, err, tc.error)
					require.Nil(t, response)
					retry, err := respond(signature)
					require.NoError(t, err)
					require.Equal(t, first, retry)
				})
			}
		})
	}
}

func TestCosiSigningReconstructsEffectiveRandom(t *testing.T) {
	for _, api := range []string{"signature", "nonce"} {
		t.Run(api, func(t *testing.T) {
			keys, publics, message := testCosiSigningPair(t)
			leaderNonce := CosiCommitNonce(RandReader())
			random1, random2 := testCosiKey(55), testCosiKey(56)
			followerNonce := newCosiNonce(&random1, &random2)
			signature := testCosiAggregateCommitment(t, map[int]*CosiCommitment{
				0: leaderNonce.Public(), 1: followerNonce.Public(),
			}, publics, message)
			leaderResponse, err := leaderNonce.Response(signature, keys[0], publics, message)
			require.NoError(t, err)

			wire := &CosiSignature{Mask: signature.Mask}
			copy(wire.Signature[32:], leaderResponse[:])
			wire.SetRandoms(signature.Randoms())
			original := wire.Signature
			respond := func(c *CosiSignature) (*[32]byte, error) {
				if api == "nonce" {
					return followerNonce.Response(c, keys[1], publics, message)
				}
				return c.Response(keys[1], &random1, &random2, publics, message)
			}

			// Concurrent callers share a wire signature whose effective random is unset.
			// Signing must reconstruct it before hashing, without mutating that signature.
			type result struct {
				response *[32]byte
				err      error
			}
			const callers = 8
			start := make(chan struct{})
			results := make(chan result, callers)
			for range callers {
				go func() {
					<-start
					response, err := respond(wire)
					results <- result{response, err}
				}()
			}
			close(start)
			var followerResponse *[32]byte
			for range callers {
				result := <-results
				require.NoError(t, result.err)
				require.NoError(t, signature.VerifyResponse(publics, 1, result.response, message))
				if followerResponse == nil {
					followerResponse = result.response
				}
				require.Equal(t, followerResponse, result.response)
			}
			require.Equal(t, original, wire.Signature)

			// A retry with the explicit effective random must bind to the same session.
			retry, err := respond(signature)
			require.NoError(t, err)
			require.Equal(t, followerResponse, retry)
			require.NoError(t, signature.AggregateResponse(publics, map[int]*[32]byte{
				0: leaderResponse, 1: followerResponse,
			}, message, true))
			require.NoError(t, signature.FullVerify(publics, 2, message))
		})
	}
}

func TestCosiAggregateSigning(t *testing.T) {
	require := require.New(t)

	keys := make([]*Key, 31)
	publics := make([]*Key, len(keys))
	for i := range keys {
		seed := Blake3Hash(fmt.Appendf(nil, "%d", i))
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
	randoms := make(map[int]*CosiCommitment)
	randKeys1 := make([]*Key, len(keys)*2/3+1)
	randKeys2 := make([]*Key, len(keys)*2/3+1)
	masks := make([]int, 0)
	commit := func(signer, slot int) {
		r1 := cosiCommit(randReader)
		r2 := cosiCommit(randReader)
		randKeys1[slot] = r1
		randKeys2[slot] = r2
		randoms[signer] = &CosiCommitment{rPub1: r1.Public(), rPub2: r2.Public()}
		masks = append(masks, signer)
	}
	for i := range 7 {
		commit(i, i)
	}
	for i := 10; i < len(randKeys1)+3; i++ {
		commit(i, i-3)
	}
	require.Len(masks, len(randoms))

	cosi, err := CosiAggregateCommitment(randoms, publics, message)
	require.Nil(err)
	require.Equal(masks, cosi.Keys())

	responses := make(map[int]*[32]byte)
	for i := 0; i < len(masks); i++ {
		s, err := cosi.Response(keys[masks[i]], randKeys1[i], randKeys2[i], publics, message)
		require.Nil(err)
		responses[masks[i]] = s
		err = cosi.VerifyResponse(publics, masks[i], s, message)
		require.Nil(err)
	}

	err = cosi.AggregateResponse(publics, responses, message, true)
	require.Nil(err)

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
