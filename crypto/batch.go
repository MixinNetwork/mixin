package crypto

import (
	"crypto/ed25519"

	"github.com/hdevalence/ed25519consensus"
)

func BatchVerify(msg []byte, keys []*Key, sigs []*Signature) bool {
	if len(keys) != len(sigs) {
		return false
	}
	if len(keys) == 1 {
		return keys[0].Verify(msg, *sigs[0])
	}
	verifier := ed25519consensus.NewBatchVerifier()
	for i := range keys {
		k, s := keys[i], sigs[i]
		verifier.Add(ed25519.PublicKey(k[:]), msg, s[:])
	}
	return verifier.Verify()
}
