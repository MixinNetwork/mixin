package crypto

import (
	"errors"
	"sync"
)

var ErrCosiNonceReuse = errors.New("cosi nonce reuse with a different challenge")

// CosiNonce is an opaque handle to a single-use Schnorr nonce pair. Copies of
// the handle share the same state and lock. Reusing the same nonce for two
// different aggregate challenges reveals the long-term private key, so the
// first response permanently binds this nonce to one challenge. An identical
// retry returns the cached response without touching the nonce again.
type CosiNonce struct {
	state *nonce
}

type nonce struct {
	sync.Mutex
	random1    *Key
	random2    *Key
	commitment CosiCommitment
	challenge  [64]byte
	response   [32]byte
	used       bool
}

func newCosiNonce(random1, random2 *Key) *CosiNonce {
	if random1 == nil || random2 == nil {
		panic("nil cosi nonce")
	}
	return &CosiNonce{state: &nonce{
		random1: random1,
		random2: random2,
		commitment: CosiCommitment{
			rPub1: random1.Public(),
			rPub2: random2.Public(),
		},
	}}
}

func (n *CosiNonce) Public() *CosiCommitment {
	commitment := n.state.commitment
	return &commitment
}

func (n *CosiNonce) Response(signature *CosiSignature, private *Key, publics []*Key, message Hash) (*[32]byte, error) {
	return n.state.respond(signature, private, publics, message)
}

func (n *nonce) respond(signature *CosiSignature, private *Key, publics []*Key, message Hash) (*[32]byte, error) {
	coefficient, challenge, err := signature.signingScalars(publics, message)
	if err != nil {
		return nil, err
	}
	// One nonce pair serves exactly one session: the coefficient binds the
	// aggregate randoms, the challenge the effective random and message.
	var challengeBytes [64]byte
	copy(challengeBytes[:32], coefficient.Bytes())
	copy(challengeBytes[32:], challenge.Bytes())

	n.Lock()
	defer n.Unlock()

	if n.used {
		if n.challenge != challengeBytes {
			return nil, ErrCosiNonceReuse
		}
		response := n.response
		return &response, nil
	}

	response := cosiResponse(private, n.random1, n.random2, coefficient, challenge)
	n.challenge = challengeBytes
	n.response = *response
	n.used = true
	for i := range n.random1 {
		n.random1[i] = 0
	}
	for i := range n.random2 {
		n.random2[i] = 0
	}
	n.random1 = nil
	n.random2 = nil

	cached := n.response
	return &cached, nil
}
