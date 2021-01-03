package zcash

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	zec := "c996abc9-d94e-4494-b1cf-2a3fd3ac5714"
	tx := "30f305889eab065bb5c85e724df9ffb1c8da7f22259c583cf874fbd6ec681b8a"
	addrMain := "t1NsuW4Xpz3GQUzt3BTZAxN6k4svKfWXgni"

	assert.Nil(VerifyAssetKey(zec))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(zec)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(zec))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(zec))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("c996abc9-d94e-4494-b1cf-2a3fd3ac5714")), GenerateAssetId(zec))
	assert.Equal(crypto.NewHash([]byte("c996abc9-d94e-4494-b1cf-2a3fd3ac5714")), ZcashChainId)
	assert.Equal(crypto.NewHash([]byte(ZcashChainBase)), ZcashChainId)
}
