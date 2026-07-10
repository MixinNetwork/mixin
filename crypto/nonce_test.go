package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func testCosiKey(seed byte) Key {
	return NewKeyFromSeed(bytes.Repeat([]byte{seed}, 64))
}

func testCosiAggregateCommitment(t *testing.T, commitments map[int]*Key) *CosiSignature {
	t.Helper()

	signature, err := CosiAggregateCommitment(commitments)
	require.NoError(t, err)
	return signature
}

func TestCosiNonceIdenticalRetry(t *testing.T) {
	private := testCosiKey(1)
	public := private.Public()
	publics := []*Key{&public}
	random := testCosiKey(2)
	nonce := newCosiNonce(&random)
	commitment := nonce.Public()
	signature := testCosiAggregateCommitment(t, map[int]*Key{0: &commitment})
	message := Blake3Hash([]byte("identical cosi challenge"))

	first, err := nonce.Response(signature, &private, publics, message)
	require.NoError(t, err)
	require.NoError(t, signature.VerifyResponse(publics, 0, first, message))
	require.Nil(t, nonce.state.random)
	require.Equal(t, Key{}, random)

	retry, err := nonce.Response(signature, &private, publics, message)
	require.NoError(t, err)
	require.Equal(t, first, retry)
	require.NoError(t, signature.VerifyResponse(publics, 0, retry, message))
}

func TestCosiNonceRejectsDifferentChallenge(t *testing.T) {
	private := testCosiKey(3)
	public := private.Public()
	peerPublic := testCosiKey(4).Public()
	publics := []*Key{&public, &peerPublic}
	random := testCosiKey(5)
	nonce := newCosiNonce(&random)
	commitment := nonce.Public()
	peerRandom1 := testCosiKey(6).Public()
	peerRandom2 := testCosiKey(7).Public()
	firstSignature := testCosiAggregateCommitment(t, map[int]*Key{
		0: &commitment,
		1: &peerRandom1,
	})
	secondSignature := testCosiAggregateCommitment(t, map[int]*Key{
		0: &commitment,
		1: &peerRandom2,
	})
	message := Blake3Hash([]byte("different cosi challenges"))

	firstChallenge, err := firstSignature.Challenge(publics, message)
	require.NoError(t, err)
	secondChallenge, err := secondSignature.Challenge(publics, message)
	require.NoError(t, err)
	require.NotEqual(t, firstChallenge.Bytes(), secondChallenge.Bytes())

	response, err := nonce.Response(firstSignature, &private, publics, message)
	require.NoError(t, err)
	require.NoError(t, firstSignature.VerifyResponse(publics, 0, response, message))

	response, err = nonce.Response(secondSignature, &private, publics, message)
	require.ErrorIs(t, err, ErrCosiNonceReuse)
	require.Nil(t, response)
}

func TestCosiNonceConcurrentChallenges(t *testing.T) {
	private := testCosiKey(8)
	public := private.Public()
	peerPublic := testCosiKey(9).Public()
	publics := []*Key{&public, &peerPublic}
	random := testCosiKey(10)
	nonce := newCosiNonce(&random)
	commitment := nonce.Public()
	peerRandom1 := testCosiKey(11).Public()
	peerRandom2 := testCosiKey(12).Public()
	signatures := []*CosiSignature{
		testCosiAggregateCommitment(t, map[int]*Key{0: &commitment, 1: &peerRandom1}),
		testCosiAggregateCommitment(t, map[int]*Key{0: &commitment, 1: &peerRandom2}),
	}
	message := Blake3Hash([]byte("concurrent cosi challenges"))

	type result struct {
		signature *CosiSignature
		response  *[32]byte
		err       error
	}
	start := make(chan struct{})
	results := make(chan result, len(signatures))
	for _, signature := range signatures {
		go func() {
			<-start
			response, err := nonce.Response(signature, &private, publics, message)
			results <- result{signature: signature, response: response, err: err}
		}()
	}
	close(start)

	var succeeded, rejected int
	for range signatures {
		result := <-results
		if result.err == nil {
			succeeded++
			require.NotNil(t, result.response)
			require.NoError(t, result.signature.VerifyResponse(publics, 0, result.response, message))
			continue
		}
		rejected++
		require.ErrorIs(t, result.err, ErrCosiNonceReuse)
		require.Nil(t, result.response)
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, rejected)
}

func TestCosiNonceCopiesShareState(t *testing.T) {
	private := testCosiKey(13)
	public := private.Public()
	peerPublic := testCosiKey(14).Public()
	publics := []*Key{&public, &peerPublic}
	random := testCosiKey(15)
	nonce := newCosiNonce(&random)
	nonceCopy := *nonce
	commitment := nonce.Public()
	peerRandom1 := testCosiKey(16).Public()
	peerRandom2 := testCosiKey(17).Public()
	firstSignature := testCosiAggregateCommitment(t, map[int]*Key{
		0: &commitment,
		1: &peerRandom1,
	})
	secondSignature := testCosiAggregateCommitment(t, map[int]*Key{
		0: &commitment,
		1: &peerRandom2,
	})
	message := Blake3Hash([]byte("copied cosi nonce handle"))

	response, err := nonce.Response(firstSignature, &private, publics, message)
	require.NoError(t, err)
	require.NoError(t, firstSignature.VerifyResponse(publics, 0, response, message))

	response, err = nonceCopy.Response(secondSignature, &private, publics, message)
	require.ErrorIs(t, err, ErrCosiNonceReuse)
	require.Nil(t, response)
}
