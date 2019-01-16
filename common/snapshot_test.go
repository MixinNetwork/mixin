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
	tx.AddScriptOutput(accounts, script, NewInteger(20000))

	s := &Snapshot{}
	s.Transaction = &SignedTransaction{
		Transaction: *tx,
	}
	assert.Len(s.Signatures, 0)
	assert.Len(s.Payload(), 428)

	seed := make([]byte, 64)
	rand.Read(seed)
	key := crypto.NewKeyFromSeed(seed)
	s.Sign(key)
	assert.Len(s.Signatures, 1)
	assert.Len(s.Payload(), 428)
	assert.False(s.CheckSignature(key))
	assert.True(s.CheckSignature(key.Public()))
	s.Sign(key)
	assert.Len(s.Signatures, 1)
	assert.Len(s.Payload(), 428)
	assert.False(s.CheckSignature(key))
	assert.True(s.CheckSignature(key.Public()))
}
