package crypto

import (
	"crypto/sha512"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto/edwards25519"
)

type CosiSignature struct {
	Signature Signature
	Mask      uint64
}

func CosiCommit(privateKey *Key, message []byte) *Key {
	var digest1, messageDigest [64]byte

	h := sha512.New()
	h.Write(privateKey[:32])
	h.Sum(digest1[:0])
	h.Reset()
	h.Write(digest1[32:])
	h.Write(message)
	h.Sum(messageDigest[:0])

	r := NewKeyFromSeed(messageDigest[:])
	return &r
}

func CosiAggregateCommitment(Rs []Key, masks []int) (*CosiSignature, error) {
	if len(Rs) != len(masks) {
		return nil, fmt.Errorf("invalid cosi commitments and masks %d %d", len(Rs), len(masks))
	}
	var encodedR *Key
	var cosi CosiSignature
	for i, R := range Rs {
		if encodedR == nil {
			encodedR = &R
		} else {
			encodedR = KeyAddPub(encodedR, &R)
		}
		err := cosi.Mark(masks[i])
		if err != nil {
			return nil, err
		}
	}
	copy(cosi.Signature[:32], encodedR[:])
	return &cosi, nil
}

func (c *CosiSignature) Challenge(publics []Key, message []byte) (*[32]byte, error) {
	var hramDigest [64]byte
	var hramDigestReduced [32]byte
	R := c.Signature[:32]
	A, err := c.AggregatePublicKey(publics)
	if err != nil {
		return nil, err
	}
	h := sha512.New()
	h.Write(R)
	h.Write(A[:])
	h.Write(message)
	h.Sum(hramDigest[:0])
	edwards25519.ScReduce(&hramDigestReduced, &hramDigest)
	return &hramDigestReduced, nil
}

func (c *CosiSignature) Response(privateKey *Key, publics []Key, message []byte) (*[32]byte, error) {
	r := CosiCommit(privateKey, message)
	messageDigestReduced := [32]byte(*r)
	expandedSecretKey := [32]byte(*privateKey)
	hramDigestReduced, err := c.Challenge(publics, message)
	if err != nil {
		return nil, err
	}
	var s [32]byte
	edwards25519.ScMulAdd(&s, hramDigestReduced, &expandedSecretKey, &messageDigestReduced)
	return &s, nil
}

func AggregateResponse(responses []*[32]byte) *[32]byte {
	var S *[32]byte
	for _, s := range responses {
		edwards25519.ScAdd(S, S, s)
	}
	return S
}

func (c *CosiSignature) Mark(i int) error {
	if i >= 64 {
		return fmt.Errorf("invalid cosi signature mask index %d", i)
	}
	c.Mask ^= (1 << uint64(i))
	return nil
}

func (c *CosiSignature) Keys() []int {
	keys := make([]int, 0)
	for i := uint64(0); i < 64; i++ {
		mask := uint64(1) << i
		if c.Mask&mask == mask {
			keys = append(keys, int(i))
		}
	}
	return keys
}

func (c *CosiSignature) AggregatePublicKey(publics []Key) (*Key, error) {
	var key *Key
	for i := range c.Keys() {
		if i >= len(publics) {
			return nil, fmt.Errorf("invalid cosi signature mask index %d/%d", i, len(publics))
		}
		k := publics[i]
		if key == nil {
			key = &k
		} else {
			key = KeyAddPub(key, &k)
		}
	}
	return key, nil
}
