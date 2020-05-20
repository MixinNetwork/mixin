package crypto

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"

	"github.com/MixinNetwork/mixin/crypto/ed25519/edwards25519"
)

type CosiSignature struct {
	Signature   Signature
	Mask        uint64
	commitments map[int]*Key
}

func CosiCommit(randReader io.Reader) *Key {
	var messageDigest [64]byte
	n, err := randReader.Read(messageDigest[:])
	if err != nil {
		panic(err)
	}
	if n != len(messageDigest) {
		panic(fmt.Errorf("rand read %d %d", len(messageDigest), n))
	}
	r := NewKeyFromSeed(messageDigest[:])
	return &r
}

func CosiAggregateCommitment(randoms map[int]*Key) (*CosiSignature, error) {
	var encodedR *Key
	cosi := CosiSignature{
		commitments: make(map[int]*Key),
	}
	for i, R := range randoms {
		if encodedR == nil {
			encodedR = R
		} else {
			encodedR = KeyAddPub(encodedR, R)
		}
		err := cosi.Mark(i)
		if err != nil {
			return nil, err
		}
		cosi.commitments[i] = R
	}
	copy(cosi.Signature[:32], encodedR[:])
	return &cosi, nil
}

func (c *CosiSignature) AggregateResponse(publics []*Key, responses map[int]*[32]byte, message []byte, strict bool) error {
	var S *[32]byte
	var keys []*Key
	for _, i := range c.Keys() {
		if i >= len(publics) {
			return fmt.Errorf("invalid cosi signature mask index %d/%d", i, len(publics))
		}
		if responses[i] == nil {
			return fmt.Errorf("invalid cosi signature responses with missing key %d", i)
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
		if c.commitments[i] == nil {
			return fmt.Errorf("invalid cosi signature response %s", hex.EncodeToString(s[:]))
		}
		if strict {
			var sig Signature
			copy(sig[:32], c.commitments[i][:])
			copy(sig[32:], s[:])
			valid := publics[i].VerifyWithChallenge(message, sig, challenge)
			if !valid {
				return fmt.Errorf("invalid cosi signature response %s", hex.EncodeToString(s[:]))
			}
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

func (c *CosiSignature) Response(privateKey, random *Key, publics []*Key, message []byte) ([32]byte, error) {
	var s [32]byte

	hramDigestReduced, err := c.Challenge(publics, message)
	if err != nil {
		return s, err
	}

	messageDigestReduced := [32]byte(*random)
	expandedSecretKey := [32]byte(*privateKey)
	edwards25519.ScMulAdd(&s, &hramDigestReduced, &expandedSecretKey, &messageDigestReduced)
	return s, nil
}

func (c *CosiSignature) VerifyResponse(publics []*Key, signer int, s *[32]byte, message []byte) error {
	var a, R *Key
	for _, k := range c.Keys() {
		if k >= len(publics) {
			return fmt.Errorf("invalid cosi signature mask index %d/%d", k, len(publics))
		}
		if k == signer {
			a = publics[k]
			R = c.commitments[k]
		}
	}
	if R == nil {
		return fmt.Errorf("invalid cosi signature mask index %d", signer)
	}
	challenge, err := c.Challenge(publics, message)
	if err != nil {
		return err
	}
	var sig Signature
	copy(sig[:32], R[:])
	copy(sig[32:], s[:])
	valid := a.VerifyWithChallenge(message, sig, challenge)
	if !valid {
		return fmt.Errorf("invalid cosi signature response %s", sig)
	}
	return nil
}

func (c *CosiSignature) Mark(i int) error {
	if i >= 64 || i < 0 {
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

func (c *CosiSignature) FullVerify(publics []*Key, threshold int, message []byte) error {
	if !c.ThresholdVerify(threshold) {
		return fmt.Errorf("cosi.FullVerify publics %d threshold %d keys %d", len(publics), threshold, len(c.Keys()))
	}
	A, err := c.AggregatePublicKey(publics)
	if err != nil {
		return fmt.Errorf("cosi.FullVerify AggregatePublicKey %s", err.Error())
	}
	if !A.Verify(message, c.Signature) {
		return fmt.Errorf("cosi.FullVerify signature verify failed")
	}
	return nil
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
