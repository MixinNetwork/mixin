package crypto

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/MixinNetwork/mixin/crypto/ed25519/edwards25519"
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

	var messageDigestReduced [32]byte
	edwards25519.ScReduce(&messageDigestReduced, &messageDigest)
	var R edwards25519.ExtendedGroupElement
	edwards25519.GeScalarMultBase(&R, &messageDigestReduced)

	var encodedR [32]byte
	R.ToBytes(&encodedR)

	pub := privateKey.Public()
	h.Reset()
	h.Write(encodedR[:])
	h.Write(pub[:])
	h.Write(message)
	h.Sum(hramDigest[:0])
	var hramDigestReduced [32]byte
	edwards25519.ScReduce(&hramDigestReduced, &hramDigest)

	var s [32]byte
	edwards25519.ScMulAdd(&s, &hramDigestReduced, &expandedSecretKey, &messageDigestReduced)

	var signature Signature
	copy(signature[:], encodedR[:])
	copy(signature[32:], s[:])

	return signature
}

func (publicKey *Key) VerifyWithChallenge(message []byte, sig Signature, hReduced [32]byte) bool {
	var A edwards25519.ExtendedGroupElement
	var publicKeyBytes [32]byte
	copy(publicKeyBytes[:], publicKey[:])
	if !A.FromBytes(&publicKeyBytes) {
		return false
	}
	edwards25519.FeNeg(&A.X, &A.X)
	edwards25519.FeNeg(&A.T, &A.T)

	var R edwards25519.ProjectiveGroupElement
	var s [32]byte
	copy(s[:], sig[32:])

	if !edwards25519.ScMinimal(&s) {
		return false
	}

	edwards25519.GeDoubleScalarMultVartime(&R, &hReduced, &A, &s)

	var checkR [32]byte
	R.ToBytes(&checkR)
	return bytes.Equal(sig[:32], checkR[:])
}

func (publicKey *Key) Verify(message []byte, sig Signature) bool {
	h := sha512.New()
	h.Write(sig[:32])
	h.Write(publicKey[:])
	h.Write(message)
	var digest [64]byte
	h.Sum(digest[:0])

	var hReduced [32]byte
	edwards25519.ScReduce(&hReduced, &digest)

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
