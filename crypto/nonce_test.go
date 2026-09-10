package crypto

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func testCosiKey(seed byte) Key {
	return NewKeyFromSeed(bytes.Repeat([]byte{seed}, 64))
}

func testCosiCommitmentPair(seed byte) *CosiCommitment {
	r1 := testCosiKey(seed)
	r2 := testCosiKey(seed + 100)
	return &CosiCommitment{rPub1: r1.Public(), rPub2: r2.Public()}
}

func testCosiAggregateCommitment(t *testing.T, commitments map[int]*CosiCommitment, publics []*Key, message Hash) *CosiSignature {
	t.Helper()

	signature, err := CosiAggregateCommitment(commitments, publics, message)
	require.NoError(t, err)
	return signature
}

func TestCosiNonceIdenticalRetry(t *testing.T) {
	private := testCosiKey(1)
	public := private.Public()
	publics := []*Key{&public}
	random1 := testCosiKey(2)
	random2 := testCosiKey(102)
	nonce := newCosiNonce(&random1, &random2)
	message := Blake3Hash([]byte("identical cosi challenge"))
	signature := testCosiAggregateCommitment(t, map[int]*CosiCommitment{0: nonce.Public()}, publics, message)

	first, err := nonce.Response(signature, &private, publics, message)
	require.NoError(t, err)
	require.NoError(t, signature.VerifyResponse(publics, 0, first, message))
	require.Nil(t, nonce.state.random1)
	require.Nil(t, nonce.state.random2)
	require.Equal(t, Key{}, random1)
	require.Equal(t, Key{}, random2)

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
	random1 := testCosiKey(5)
	random2 := testCosiKey(105)
	nonce := newCosiNonce(&random1, &random2)
	peerRandom1 := testCosiCommitmentPair(6)
	peerRandom2 := testCosiCommitmentPair(7)
	message := Blake3Hash([]byte("different cosi challenges"))
	firstSignature := testCosiAggregateCommitment(t, map[int]*CosiCommitment{
		0: nonce.Public(),
		1: peerRandom1,
	}, publics, message)
	secondSignature := testCosiAggregateCommitment(t, map[int]*CosiCommitment{
		0: nonce.Public(),
		1: peerRandom2,
	}, publics, message)

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
	random1 := testCosiKey(10)
	random2 := testCosiKey(110)
	nonce := newCosiNonce(&random1, &random2)
	peerRandom1 := testCosiCommitmentPair(11)
	peerRandom2 := testCosiCommitmentPair(12)
	message := Blake3Hash([]byte("concurrent cosi challenges"))
	signatures := []*CosiSignature{
		testCosiAggregateCommitment(t, map[int]*CosiCommitment{0: nonce.Public(), 1: peerRandom1}, publics, message),
		testCosiAggregateCommitment(t, map[int]*CosiCommitment{0: nonce.Public(), 1: peerRandom2}, publics, message),
	}

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
	random1 := testCosiKey(15)
	random2 := testCosiKey(115)
	nonce := newCosiNonce(&random1, &random2)
	nonceCopy := *nonce
	peerRandom1 := testCosiCommitmentPair(16)
	peerRandom2 := testCosiCommitmentPair(17)
	message := Blake3Hash([]byte("copied cosi nonce handle"))
	firstSignature := testCosiAggregateCommitment(t, map[int]*CosiCommitment{
		0: nonce.Public(),
		1: peerRandom1,
	}, publics, message)
	secondSignature := testCosiAggregateCommitment(t, map[int]*CosiCommitment{
		0: nonce.Public(),
		1: peerRandom2,
	}, publics, message)

	response, err := nonce.Response(firstSignature, &private, publics, message)
	require.NoError(t, err)
	require.NoError(t, firstSignature.VerifyResponse(publics, 0, response, message))

	response, err = nonceCopy.Response(secondSignature, &private, publics, message)
	require.ErrorIs(t, err, ErrCosiNonceReuse)
	require.Nil(t, response)
}
