package cosmos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	atom := "uatom"
	tx := "c9698260bab4095df25a228a3d855918de38a9e0c57d7a137de18b4c141f26ee"
	addrMain := "cosmos14xwf5zcf0qk2t8vuqtr0zv9yt9g85dust0u68d"

	assert.Nil(VerifyAssetKey(atom))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(atom)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(atom))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(atom))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("7397e9f1-4e42-4dc8-8a3b-171daaadd436")), GenerateAssetId(atom))
	assert.Equal(crypto.NewHash([]byte("7397e9f1-4e42-4dc8-8a3b-171daaadd436")), CosmosChainId)
	assert.Equal(crypto.NewHash([]byte(CosmosChainBase)), CosmosChainId)
}
