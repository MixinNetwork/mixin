package namecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	nmc := "f8b77dc0-46fd-4ea1-9821-587342475869"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "NCjrV4CWpSr73mfYADbiujetMB3F3VrDWc"

	assert.Nil(VerifyAssetKey(nmc))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(nmc)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(nmc))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(nmc))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("f8b77dc0-46fd-4ea1-9821-587342475869")), GenerateAssetId(nmc))
	assert.Equal(crypto.NewHash([]byte("f8b77dc0-46fd-4ea1-9821-587342475869")), NamecoinChainId)
	assert.Equal(crypto.NewHash([]byte(NamecoinChainBase)), NamecoinChainId)
}
