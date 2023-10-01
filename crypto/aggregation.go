package crypto

import (
	"fmt"

	"filippo.io/edwards25519"
)

func aggregatePublicKey(publics []*Key, signers []int) (*Key, error) {
	P := edwards25519.NewIdentityPoint()
	for _, i := range signers {
		if i >= len(publics) {
			return nil, fmt.Errorf("invalid aggregation singer index %d/%d", i, len(publics))
		}
		p, err := edwards25519.NewIdentityPoint().SetBytes(publics[i][:])
		if err != nil {
			return nil, err
		}
		P = P.Add(P, p)
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
