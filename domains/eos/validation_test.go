package eos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	eos := "eosio.token:EOS"
	usdt := "tethertether:USDT"
	tx := "197be13b8d572ae4c83fe2bc60e87ac8993896242bb486790fd4378f88d8d961"

	assert.Nil(VerifyAssetKey(eos))
	assert.Nil(VerifyAssetKey(usdt))
	assert.NotNil(VerifyAssetKey("eosio.token"))
	assert.NotNil(VerifyAssetKey("eosio.token.2"))
	assert.NotNil(VerifyAssetKey("eos.io.token"))
	assert.NotNil(VerifyAssetKey("eosio:EOSABCDEFG"))
	assert.NotNil(VerifyAssetKey("eosio.token:EOS:2"))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(eos)))

	assert.Nil(VerifyAddress("eosio.token"))
	assert.Nil(VerifyAddress("tethertether"))
	assert.NotNil(VerifyAddress("eosio.token6"))
	assert.NotNil(VerifyAddress("Eosio.token"))
	assert.NotNil(VerifyAddress("eos.io.token"))
	assert.NotNil(VerifyAddress("."))
	assert.NotNil(VerifyAddress(".token"))
	assert.NotNil(VerifyAddress("eosio."))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(eos))
	assert.NotNil(VerifyTransactionHash(tx[2:]))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("6cfe566e-4aad-470b-8c9a-2fd35b49c68d")), GenerateAssetId(eos))
	assert.Equal(crypto.NewHash([]byte("5dac5e28-ad13-31ea-869f-41770dfcee09")), GenerateAssetId(usdt))
	assert.Equal(crypto.NewHash([]byte("6cfe566e-4aad-470b-8c9a-2fd35b49c68d")), EOSChainId)
}
