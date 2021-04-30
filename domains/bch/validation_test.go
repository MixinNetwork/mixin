package bch

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	bch := "fd11b6e3-0b87-41f1-a41f-f0e9b49e5bf0"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "19q6XbBBYLhxnQGxWeS3fiehV5huV8bAZd"
	addrNew := "bitcoincash:pp8skudq3x5hzw8ew7vzsw8tn4k8wxsqsv0lt0mf3g"

	assert.Nil(VerifyAssetKey(bch))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(bch)))

	assert.Nil(VerifyAddress(addrMain))
	assert.Nil(VerifyAddress(addrNew))
	assert.NotNil(VerifyAddress(bch))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(bch))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("fd11b6e3-0b87-41f1-a41f-f0e9b49e5bf0")), GenerateAssetId(bch))
	assert.Equal(crypto.NewHash([]byte("fd11b6e3-0b87-41f1-a41f-f0e9b49e5bf0")), BitcoinCashChainId)
	assert.Equal(crypto.NewHash([]byte(BitcoinCashChainBase)), BitcoinCashChainId)
}
