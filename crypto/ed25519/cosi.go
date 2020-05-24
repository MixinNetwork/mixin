package ed25519

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/crypto/ed25519/edwards25519"
)

func (f keyFactory) CosiAggregateCommitments(cosi *crypto.CosiSignature, commitments map[int]*crypto.Commitment) error {
	var aggRandom crypto.PublicKey
	for i, index := range cosi.Keys() {
		R, ok := commitments[index]
		if !ok {
			return fmt.Errorf("commitment %d not found", i)
		}

		if i == 0 {
			aggRandom = crypto.Key(*R).AsPublicKeyOrPanic()
		} else {
			aggRandom = aggRandom.AddPublic(crypto.Key(*R).AsPublicKeyOrPanic())
		}
	}

	var (
		s  crypto.Signature
		RK = aggRandom.Key()
	)
	copy(s[:32], RK[:])
	cosi.Signatures = []crypto.Signature{s}
	return nil
}

func (f keyFactory) UpdateSignatureCommitment(sig *crypto.Signature, commitment *crypto.Commitment) {
	copy(sig[:32], commitment[:])
}

func (f keyFactory) DumpSignatureResponse(sig *crypto.Signature) *crypto.Response {
	var response crypto.Response
	copy(response[:], sig[32:])
	return &response
}

func (f keyFactory) LoadResponseSignature(cosi *crypto.CosiSignature, commitment *crypto.Commitment, response *crypto.Response) (*crypto.Signature, error) {
	if len(response) != 32 {
		return nil, fmt.Errorf("invalid signature response size: %d", len(response))
	}
	var sig crypto.Signature
	copy(sig[:32], commitment[:])
	copy(sig[32:], response[:])
	return &sig, nil
}

func (f keyFactory) CosiDumps(cosi *crypto.CosiSignature) (data []byte) {
	var sig crypto.Signature
	if len(cosi.Signatures) > 0 {
		sig = cosi.Signatures[0]
	}
	mask := make([]byte, 8)
	binary.BigEndian.PutUint64(mask, cosi.Mask)
	data = append(sig[:], mask...)
	return
}

func (f keyFactory) CosiLoads(cosi *crypto.CosiSignature, data []byte) (rest []byte, err error) {
	if len(data) < 72 {
		err = fmt.Errorf("invalid challenge message size %d", len(data))
		return
	}

	var sig crypto.Signature
	copy(sig[:], data[:64])
	cosi.Mask = binary.BigEndian.Uint64(data[64:72])
	cosi.Signatures = []crypto.Signature{sig}
	rest = data[72:]
	return
}

func (f keyFactory) CosiChallenge(cosi *crypto.CosiSignature, publics map[int]crypto.PublicKey, message []byte) ([32]byte, error) {
	var (
		P      crypto.PublicKey
		R      crypto.PublicKey
		inited bool
	)

	if len(cosi.Signatures) != 1 {
		return [32]byte{}, fmt.Errorf("invalid signature size")
	}

	{
		sig := cosi.Signatures[0]

		var rand Key
		copy(rand[:], sig[:32])
		R = rand
	}

	for _, i := range cosi.Keys() {
		pub, ok := publics[i]
		if !ok {
			return [32]byte{}, fmt.Errorf("public key %d not found", i)
		}

		if !inited {
			P = pub
			inited = true
		} else {
			P = P.AddPublic(pub)
		}
	}

	return P.Challenge(R, message), nil
}

func (f keyFactory) CosiAggregateSignature(cosi *crypto.CosiSignature, node int, sig *crypto.Signature) error {
	if len(cosi.Signatures) != 1 {
		return fmt.Errorf("invalid cosignature size")
	}

	var (
		cs = &cosi.Signatures[0]
		s1 [32]byte
		s2 [32]byte
	)

	copy(s1[:], cs[32:])
	copy(s2[:], sig[32:])
	edwards25519.ScAdd(&s1, &s1, &s2)
	copy(cs[32:], s1[:])
	return nil
}

func (f keyFactory) CosiFullVerify(publics map[int]crypto.PublicKey, message []byte, cosi *crypto.CosiSignature) bool {
	if len(cosi.Signatures) != 1 {
		return false
	}

	var (
		pub    crypto.PublicKey
		sig    = cosi.Signatures[0]
		inited bool
	)

	for _, P := range publics {
		if !inited {
			pub = P
			inited = true
		} else {
			pub = pub.AddPublic(P)
		}
	}
	return pub.Verify(message, &sig)
}
