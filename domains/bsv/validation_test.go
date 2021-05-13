package bsv

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	bsv := "574388fd-b93f-4034-a682-01c2bc095d17"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "19q6XbBBYLhxnQGxWeS3fiehV5huV8bAZd"

	assert.Nil(VerifyAssetKey(bsv))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(bsv)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(bsv))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(bsv))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("574388fd-b93f-4034-a682-01c2bc095d17")), GenerateAssetId(bsv))
	assert.Equal(crypto.NewHash([]byte("574388fd-b93f-4034-a682-01c2bc095d17")), BitcoinSVChainId)
	assert.Equal(crypto.NewHash([]byte(BitcoinSVChainBase)), BitcoinSVChainId)
}
