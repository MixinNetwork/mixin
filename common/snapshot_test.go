// +build ed25519 !custom_alg

package common

import (
	"crypto/rand"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestSnapshot(t *testing.T) {
	assert := assert.New(t)

	genesisHash := crypto.Hash{}
	script := Script{OperatorCmp, OperatorSum, 2}
	accounts := make([]Address, 0)
	for i := 0; i < 3; i++ {
		accounts = append(accounts, randomAccount())
	}

	tx := NewTransaction(XINAssetId)
	tx.AddInput(genesisHash, 0)
	tx.AddInput(genesisHash, 1)
	tx.AddRandomScriptOutput(accounts, script, NewInteger(20000))

	s := &Snapshot{Version: SnapshotVersion}
	assert.Len(s.VersionedPayload(), 133)
	assert.Equal("da2c8a9f34d14ba24a4a09dfacf9506396c48a7705152f082b5795860dad89cf", s.PayloadHash().String())

	s = &Snapshot{}
	assert.Len(s.Signatures, 0)
	assert.Len(s.VersionedPayload(), 136)
	assert.Equal("fb08f9901437365528fdca2ad2e6cea782793d82286f152d6c147e41ec078074", s.PayloadHash().String())

	seed := make([]byte, 64)
	rand.Read(seed)
	key := crypto.PrivateKeyFromSeed(seed)
	rand.Read(seed)
	pub1 := crypto.PrivateKeyFromSeed(seed).Public()
	sign(s, key)
	assert.Len(s.Signatures, 1)
	assert.Len(s.VersionedPayload(), 136)
	assert.False(checkSignature(s, pub1))
	assert.True(checkSignature(s, key.Public()))
	sign(s, key)
	assert.Len(s.Signatures, 1)
	assert.Len(s.VersionedPayload(), 136)
	assert.False(checkSignature(s, pub1))
	assert.True(checkSignature(s, key.Public()))
}

func checkSignature(s *Snapshot, pub crypto.PublicKey) bool {
	msg := s.PayloadHash()
	for _, sig := range s.Signatures {
		if pub.Verify(msg[:], sig) {
			return true
		}
	}
	return false
}

func sign(s *Snapshot, key crypto.PrivateKey) {
	msg := s.PayloadHash()
	sig, _ := key.Sign(msg[:])
	for _, o := range s.Signatures {
		if o.String() == sig.String() {
			return
		}
	}
	s.Signatures = append(s.Signatures, sig)
}
