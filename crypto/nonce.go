package crypto

import (
	"errors"
	"sync"
)

var ErrCosiNonceReuse = errors.New("cosi nonce reuse with a different challenge")

// CosiNonce owns a single-use Schnorr nonce. Reusing the same nonce for two
// different aggregate challenges reveals the long-term private key, so the
// first response permanently binds this nonce to one challenge. An identical
// retry returns the cached response without touching the nonce again.
type CosiNonce struct {
	sync.Mutex
	random     *Key
	commitment Key
	challenge  [32]byte
	response   [32]byte
	used       bool
}

func newCosiNonce(random *Key) *CosiNonce {
	if random == nil {
		panic("nil cosi nonce")
	}
	return &CosiNonce{
		random:     random,
		commitment: random.Public(),
	}
}

func (n *CosiNonce) Public() Key {
	return n.commitment
}

func (n *CosiNonce) Response(signature *CosiSignature, private *Key, publics []*Key, message Hash) (*[32]byte, error) {
	challenge, err := signature.Challenge(publics, message)
	if err != nil {
		return nil, err
	}
	var challengeBytes [32]byte
	copy(challengeBytes[:], challenge.Bytes())

	n.Lock()
	defer n.Unlock()

	if n.used {
		if n.challenge != challengeBytes {
			return nil, ErrCosiNonceReuse
		}
		response := n.response
		return &response, nil
	}

	response, err := signature.Response(private, n.random, publics, message)
	if err != nil {
		return nil, err
	}
	n.challenge = challengeBytes
	n.response = *response
	n.used = true
	for i := range n.random {
		n.random[i] = 0
	}
	n.random = nil

	cached := n.response
	return &cached, nil
}
