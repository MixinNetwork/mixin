package crypto

import (
	"bytes"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"

	"filippo.io/edwards25519"
)

type Signature [64]byte

func (s *Signature) R() []byte {
	return s[:32]
}

func (s *Signature) S() []byte {
	return s[32:]
}

func (privateKey *Key) Sign(message []byte) Signature {
	var digest1, messageDigest, hramDigest [64]byte
	var expandedSecretKey [32]byte
	copy(expandedSecretKey[:], privateKey[:])

	h := sha512.New()
	h.Write(privateKey[:32])
	h.Sum(digest1[:0])
	h.Reset()
	h.Write(digest1[32:])
	h.Write(message)
	h.Sum(messageDigest[:0])

	z := edwards25519.NewScalar().SetUniformBytes(messageDigest[:])
	R := edwards25519.NewIdentityPoint().ScalarBaseMult(z)

	var encodedR [32]byte
	copy(encodedR[:], R.Bytes())

	pub := privateKey.Public()
	h.Reset()
	h.Write(encodedR[:])
	h.Write(pub[:])
	h.Write(message)
	h.Sum(hramDigest[:0])
	x := edwards25519.NewScalar().SetUniformBytes(hramDigest[:])

	y, err := edwards25519.NewScalar().SetCanonicalBytes(privateKey[:])
	if err != nil {
		panic(privateKey.String())
	}
	s := edwards25519.NewScalar().MultiplyAdd(x, y, z)

	var signature Signature
	copy(signature[:], encodedR[:])
	copy(signature[32:], s.Bytes())

	return signature
}

// order is the order of Curve25519 in little-endian form.
var order = [4]uint64{0x5812631a5cf5d3ed, 0x14def9dea2f79cd6, 0, 0x1000000000000000}

// ScMinimal returns true if the given scalar is less than the order of the
// curve.
func ScMinimal(scalar *[32]byte) bool {
	for i := 3; ; i-- {
		v := binary.LittleEndian.Uint64(scalar[i*8:])
		if v > order[i] {
			return false
		} else if v < order[i] {
			break
		} else if i == 0 {
			return false
		}
	}

	return true
}

func (publicKey *Key) VerifyWithChallenge(message []byte, sig Signature, hReduced [32]byte) bool {
	p, err := edwards25519.NewIdentityPoint().SetBytes(publicKey[:])
	if err != nil {
		panic(err)
	}
	A := edwards25519.NewIdentityPoint().Negate(p)

	var s [32]byte
	copy(s[:], sig[32:])

	if !ScMinimal(&s) {
		return false
	}

	a, err := edwards25519.NewScalar().SetCanonicalBytes(hReduced[:])
	if err != nil {
		panic(hReduced)
	}
	b, err := edwards25519.NewScalar().SetCanonicalBytes(sig[32:])
	if err != nil {
		panic(sig)
	}
	R := edwards25519.NewIdentityPoint().VarTimeDoubleScalarBaseMult(a, A, b)
	return bytes.Equal(sig[:32], R.Bytes())
}

func (publicKey *Key) Verify(message []byte, sig Signature) bool {
	h := sha512.New()
	h.Write(sig[:32])
	h.Write(publicKey[:])
	h.Write(message)
	var digest [64]byte
	h.Sum(digest[:0])

	var hReduced [32]byte
	x := edwards25519.NewScalar().SetUniformBytes(digest[:])
	copy(hReduced[:], x.Bytes())
	return publicKey.VerifyWithChallenge(message, sig, hReduced)
}

func (s Signature) String() string {
	return hex.EncodeToString(s[:])
}

func (s Signature) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(s.String())), nil
}

func (s *Signature) UnmarshalJSON(b []byte) error {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	data, err := hex.DecodeString(string(unquoted))
	if err != nil {
		return err
	}
	if len(data) != len(s) {
		return fmt.Errorf("invalid signature length %d", len(data))
	}
	copy(s[:], data)
	return nil
}
