package crypto

import (
	"fmt"

	"filippo.io/edwards25519"
)

func aggregatePublicKey(publics []*Key, signers []int) (*Key, error) {
	P := edwards25519.NewIdentityPoint()
	prev := -1
	for _, i := range signers {
		if i <= prev {
			return nil, fmt.Errorf("invalid aggregation signer order %d <= %d", i, prev)
		}
		if i >= len(publics) {
			return nil, fmt.Errorf("invalid aggregation signer index %d/%d", i, len(publics))
		}
		p, err := decodePoint(publics[i][:])
		if err != nil {
			return nil, err
		}
		P = P.Add(P, p)
		prev = i
	}
	var key Key
	copy(key[:], P.Bytes())
	return &key, nil
}

func AggregateVerify(sig *Signature, publics []*Key, signers []int, message Hash) error {
	A, err := aggregatePublicKey(publics, signers)
	if err != nil {
		return fmt.Errorf("AggregateVerify aggregatePublicKey %v", err)
	}
	if !A.Verify(message, *sig) {
		return fmt.Errorf("AggregateVerify signature verify failed")
	}
	return nil
}
