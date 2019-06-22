package crypto

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/crypto/edwards25519"
)

type CosiSignature struct {
	Signature   Signature
	Mask        uint64
	commitments []*Key
}

func CosiCommit(privateKey *Key, publics []*Key, message []byte) *Key {
	var digest1, messageDigest [64]byte
	pub := CosiHashAggregateAllPublics(publics)

	h := sha512.New()
	h.Write(privateKey[:32])
	h.Sum(digest1[:0])
	h.Reset()
	h.Write(digest1[32:])
	h.Write(message)
	h.Write(pub[:])
	h.Sum(messageDigest[:0])

	r := NewKeyFromSeed(messageDigest[:])
	return &r
}

func CosiAggregateCommitment(randoms []*Key, masks []int) (*CosiSignature, error) {
	if len(randoms) != len(masks) {
		return nil, fmt.Errorf("invalid cosi commitments and masks %d %d", len(randoms), len(masks))
	}
	var encodedR *Key
	var cosi CosiSignature
	for i, R := range randoms {
		if encodedR == nil {
			encodedR = R
		} else {
			encodedR = KeyAddPub(encodedR, R)
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

func (c *CosiSignature) AggregateResponse(publics []*Key, responses []*[32]byte, message []byte) error {
	var S *[32]byte
	var keys []*Key
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
		valid := cosiVerifyWithChallenge(keys[i], message, sig, challenge)
		if !valid {
			return fmt.Errorf("invalid cosi signature response %s", sig)
		}
		if S == nil {
			S = s
		} else {
			edwards25519.ScAdd(S, S, s)
		}
	}
	copy(c.Signature[32:], S[:])
	return nil
}

func (c *CosiSignature) Challenge(publics []*Key, message []byte) ([32]byte, error) {
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

func (c *CosiSignature) Response(privateKey *Key, publics []*Key, message []byte) ([32]byte, error) {
	var s [32]byte
	r := CosiCommit(privateKey, publics, message)

	hramDigestReduced, err := c.Challenge(publics, message)
	if err != nil {
		return s, err
	}

	messageDigestReduced := [32]byte(*r)
	var digest [64]byte
	h := sha512.New()
	h.Write(hramDigestReduced[:])
	h.Sum(digest[:0])
	var cReduced [32]byte
	edwards25519.ScReduce(&cReduced, &digest)
	edwards25519.ScMulAdd(&messageDigestReduced, &cReduced, &messageDigestReduced, &s)

	expandedSecretKey := [32]byte(*privateKey)
	edwards25519.ScMulAdd(&s, &hramDigestReduced, &expandedSecretKey, &messageDigestReduced)
	return s, nil
}

func (c *CosiSignature) VerifyResponse(publics []*Key, signer int, s *[32]byte, message []byte) error {
	var a, R *Key
	for i, k := range c.Keys() {
		if k >= len(publics) {
			return fmt.Errorf("invalid cosi signature mask index %d/%d", k, len(publics))
		}
		if k == signer {
			a = publics[k]
			R = c.commitments[i]
		}
	}
	challenge, err := c.Challenge(publics, message)
	if err != nil {
		return err
	}
	var sig Signature
	copy(sig[:32], R[:])
	copy(sig[32:], s[:])
	valid := cosiVerifyWithChallenge(a, message, sig, challenge)
	if !valid {
		return fmt.Errorf("invalid cosi signature response %s", sig)
	}
	return nil
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

func (c *CosiSignature) AggregatePublicKey(publics []*Key) (*Key, error) {
	var key *Key
	for _, i := range c.Keys() {
		if i >= len(publics) {
			return nil, fmt.Errorf("invalid cosi signature mask index %d/%d", i, len(publics))
		}
		k := publics[i]
		if key == nil {
			key = k
		} else {
			key = KeyAddPub(key, k)
		}
	}
	return key, nil
}

func (c *CosiSignature) ThresholdVerify(threshold int) bool {
	return len(c.Keys()) >= threshold
}

func (c *CosiSignature) FullVerify(publics []*Key, threshold int, message []byte) bool {
	if !c.ThresholdVerify(threshold) {
		return false
	}
	A, err := c.AggregatePublicKey(publics)
	if err != nil {
		return false
	}
	return cosiVerify(A, message, c.Signature)
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

func CosiHashAggregateAllPublics(publics []*Key) []byte {
	var pub *Key
	for i, _ := range publics {
		k := publics[i]
		if pub == nil {
			pub = k
		} else {
			pub = KeyAddPub(pub, k)
		}
	}
	hash := NewHash(pub[:])
	return hash[:]
}

func cosiVerifyWithChallenge(publicKey *Key, message []byte, sig Signature, hramReduced [32]byte) bool {
	var A edwards25519.ExtendedGroupElement
	var publicKeyBytes [32]byte
	copy(publicKeyBytes[:], publicKey[:])
	if !A.FromBytes(&publicKeyBytes) {
		return false
	}
	edwards25519.FeNeg(&A.X, &A.X)
	edwards25519.FeNeg(&A.T, &A.T)

	var s [32]byte
	copy(s[:], sig[32:])
	if !edwards25519.ScMinimal(&s) {
		return false
	}

	var digest [64]byte
	h := sha512.New()
	h.Write(hramReduced[:])
	h.Sum(digest[:0])
	var cReduced [32]byte
	edwards25519.ScReduce(&cReduced, &digest)
	var RKey, cKey Key
	copy(RKey[:], sig[:32])
	copy(cKey[:], cReduced[:])
	Rm := KeyMultPubPriv(&RKey, &cKey)

	var R edwards25519.ProjectiveGroupElement
	edwards25519.GeDoubleScalarMultVartime(&R, &hramReduced, &A, &s)
	var checkR [32]byte
	R.ToBytes(&checkR)

	return bytes.Equal(Rm[:], checkR[:])
}

func cosiVerify(publicKey *Key, message []byte, sig Signature) bool {
	var digest [64]byte
	h := sha512.New()
	h.Write(sig[:32])
	h.Write(publicKey[:])
	h.Write(message)
	h.Sum(digest[:0])

	var hramReduced [32]byte
	edwards25519.ScReduce(&hramReduced, &digest)

	return cosiVerifyWithChallenge(publicKey, message, sig, hramReduced)
}
