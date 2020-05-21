package ed25519

import (
	"encoding/binary"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/crypto/ed25519/edwards25519"
)

func (f keyFactory) CosiInitLoad(cosi *crypto.CosiSignature, commitents map[int]crypto.PublicKey) error {
	var aggRandom crypto.PublicKey
	for i, index := range cosi.Keys() {
		R, ok := commitents[index]
		if !ok {
			return fmt.Errorf("commitment %d not found", i)
		}

		if i == 0 {
			aggRandom = R
		} else {
			aggRandom = aggRandom.AddPublic(R)
		}
	}

	var (
		s  crypto.Signature
		RK = aggRandom.Key()
	)
	copy(s[:32], RK[:])
	cosi.Signatures[0] = s
	return nil
}

func (f keyFactory) CosiDumps(cosi *crypto.CosiSignature) (data []byte, err error) {
	sig, ok := cosi.Signatures[0]
	if !ok {
		err = fmt.Errorf("invalid signature size")
		return
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
	cosi.Signatures = map[int]crypto.Signature{
		0: sig,
	}
	rest = data[72:]
	return
}

func (f keyFactory) CosiChallenge(cosi *crypto.CosiSignature, publics map[int]crypto.PublicKey, message []byte) ([32]byte, error) {
	var (
		P      crypto.PublicKey
		R      crypto.PublicKey
		inited bool
	)

	{
		sig, ok := cosi.Signatures[0]
		if !ok {
			return [32]byte{}, fmt.Errorf("invalid signature size")
		}

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

func (f keyFactory) CosiAggregateSignatures(cosi *crypto.CosiSignature, sigs map[int]crypto.Signature) error {
	sig, ok := cosi.Signatures[0]
	if !ok {
		return fmt.Errorf("invalid cosignature size")
	}

	var (
		S *[32]byte
		s [32]byte
	)
	for _, i := range cosi.Keys() {
		sig, ok := sigs[i]
		if !ok {
			return fmt.Errorf("signature %d not found", i)
		}

		if S == nil {
			S = new([32]byte)
			copy(S[:], sig[32:])
		} else {
			copy(s[:], sig[32:])
			edwards25519.ScAdd(S, S, &s)
		}
	}
	copy(sig[32:], S[:])
	cosi.Signatures[0] = sig
	return nil
}

func (f keyFactory) CosiFullVerify(publics map[int]crypto.PublicKey, message []byte, sig crypto.CosiSignature) bool {
	if len(sig.Signatures) != 1 {
		return false
	}

	var (
		pub       crypto.PublicKey
		signature crypto.Signature
		inited    bool
	)

	for _, s := range sig.Signatures {
		signature = s
	}

	for _, P := range publics {
		if !inited {
			pub = P
			inited = true
		} else {
			pub = pub.AddPublic(P)
		}
	}
	return pub.Verify(message, signature)
}
