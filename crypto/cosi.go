package crypto

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/crypto/edwards25519"
)

type CosiSignature struct {
	Signature   Signature
	Mask        uint64
	commitments []Key
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
		cosi.commitments = append(cosi.commitments, R)
	}
	copy(cosi.Signature[:32], encodedR[:])
	return &cosi, nil
}

func (c *CosiSignature) AggregateResponse(publics []Key, responses [][32]byte, message []byte) error {
	var S *[32]byte
	var keys []Key
	for _, i := range c.Keys() {
		if i >= len(publics) {
			return fmt.Errorf("invalid cosi signature mask index %d/%d", i, len(publics))
		}
		keys = append(keys, publics[i])
	}
	if len(keys) != len(responses) {
		return fmt.Errorf("invalid cosi signature responses count %d/%d", len(keys), len(responses))
	}
	challenge, err := c.Challenge(publics, message)
	if err != nil {
		return err
	}
	for i, s := range responses {
		var sig Signature
		copy(sig[:32], c.commitments[i][:])
		copy(sig[32:], s[:])
		valid := keys[i].VerifyWithChallenge(message, sig, challenge)
		if !valid {
			return fmt.Errorf("invalid cosi signature response %s", sig)
		}
		if S == nil {
			S = &s
		} else {
			edwards25519.ScAdd(S, S, &s)
		}
	}
	copy(c.Signature[32:], S[:])
	return nil
}

func (c *CosiSignature) Challenge(publics []Key, message []byte) ([32]byte, error) {
	var hramDigest [64]byte
	var hramDigestReduced [32]byte
	R := c.Signature[:32]
	A, err := c.AggregatePublicKey(publics)
	if err != nil {
		return hramDigestReduced, err
	}
	h := sha512.New()
	h.Write(R)
	h.Write(A[:])
	h.Write(message)
	h.Sum(hramDigest[:0])
	edwards25519.ScReduce(&hramDigestReduced, &hramDigest)
	return hramDigestReduced, nil
}

func (c *CosiSignature) Response(privateKey *Key, publics []Key, message []byte) ([32]byte, error) {
	var s [32]byte
	r := CosiCommit(privateKey, message)
	messageDigestReduced := [32]byte(*r)
	expandedSecretKey := [32]byte(*privateKey)
	hramDigestReduced, err := c.Challenge(publics, message)
	if err != nil {
		return s, err
	}
	edwards25519.ScMulAdd(&s, &hramDigestReduced, &expandedSecretKey, &messageDigestReduced)
	return s, nil
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

func (c CosiSignature) String() string {
	return c.Signature.String() + fmt.Sprintf("%016x", c.Mask)
}

func (c CosiSignature) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(c.String())), nil
}

func (c *CosiSignature) UnmarshalJSON(b []byte) error {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	data, err := hex.DecodeString(string(unquoted))
	if err != nil {
		return err
	}
	if len(data) != len(c.Signature)+8 {
		return fmt.Errorf("invalid signature length %d", len(data))
	}
	copy(c.Signature[:], data)
	c.Mask, err = strconv.ParseUint(unquoted[len(c.Signature)*2:], 16, 64)
	if err != nil {
		return fmt.Errorf("invalid mask data %x", unquoted[len(c.Signature)*2:])
	}
	return nil
}
