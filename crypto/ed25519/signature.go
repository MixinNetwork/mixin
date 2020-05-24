package ed25519

import (
	"bytes"
	"crypto/sha512"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/crypto/ed25519/edwards25519"
)

func (k Key) SignWithChallenge(random crypto.PrivateKey, message []byte, hReduced [32]byte) *crypto.Signature {
	var (
		messageReduced [32]byte
		s              [32]byte
		rK             = random.Key()
	)
	copy(messageReduced[:], rK[:])

	expandedSecretKey := [32]byte(k)
	edwards25519.ScMulAdd(&s, &hReduced, &expandedSecretKey, &messageReduced)

	R := random.Public().Key()
	var signature crypto.Signature
	copy(signature[:], R[:])
	copy(signature[32:], s[:])
	return &signature
}

func (k Key) Sign(message []byte) *crypto.Signature {
	var digest1, messageDigest, hramDigest [64]byte
	var expandedSecretKey [32]byte
	copy(expandedSecretKey[:], k[:])

	h := sha512.New()
	h.Write(k[:])
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

	pub := k.Public().Key()
	h.Reset()
	h.Write(encodedR[:])
	h.Write(pub[:])
	h.Write(message)
	h.Sum(hramDigest[:0])
	var hramDigestReduced [32]byte
	edwards25519.ScReduce(&hramDigestReduced, &hramDigest)

	var s [32]byte
	edwards25519.ScMulAdd(&s, &hramDigestReduced, &expandedSecretKey, &messageDigestReduced)

	var signature crypto.Signature
	copy(signature[:], encodedR[:])
	copy(signature[32:], s[:])
	return &signature
}

func (k Key) VerifyWithChallenge(message []byte, sig *crypto.Signature, hReduced [32]byte) bool {
	var (
		pubBts = [32]byte(k)
		A      edwards25519.ExtendedGroupElement
	)

	if !A.FromBytes(&pubBts) {
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

func (k Key) Verify(message []byte, sig *crypto.Signature) bool {
	var R Key
	copy(R[:], sig[:32])
	return k.VerifyWithChallenge(message, sig, k.Challenge(R, message))
}
