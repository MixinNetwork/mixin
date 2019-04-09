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

	s := &Snapshot{}
	assert.Len(s.Signatures, 0)
	assert.Len(s.Payload(), 136)

	seed := make([]byte, 64)
	rand.Read(seed)
	key := crypto.NewKeyFromSeed(seed)
	sign(s, key)
	assert.Len(s.Signatures, 1)
	assert.Len(s.Payload(), 136)
	assert.False(checkSignature(s, key))
	assert.True(checkSignature(s, key.Public()))
	sign(s, key)
	assert.Len(s.Signatures, 1)
	assert.Len(s.Payload(), 136)
	assert.False(checkSignature(s, key))
	assert.True(checkSignature(s, key.Public()))
}

func checkSignature(s *Snapshot, pub crypto.Key) bool {
	msg := s.PayloadHash()
	for _, sig := range s.Signatures {
		if pub.Verify(msg[:], *sig) {
			return true
		}
	}
	return false
}

func sign(s *Snapshot, key crypto.Key) {
	msg := s.PayloadHash()
	sig := key.Sign(msg[:])
	for _, o := range s.Signatures {
		if o.String() == sig.String() {
			return
		}
	}
	s.Signatures = append(s.Signatures, &sig)
}
