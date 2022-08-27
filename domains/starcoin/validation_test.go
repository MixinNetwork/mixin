package starcoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	stc := "0x00000000000000000000000000000001::STC::STC"
	tx := "0xa1dc5ccb8c7ebcb557c3514ed851a23e07c3311faf20b95cba8a0f5d1648ff11"
	addrMain := "0x7a86a44a5c5ed4827402dd09db4a2353"

	assert.Nil(VerifyAssetKey(stc))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(stc)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(stc))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(stc))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("c99a3779-93df-404d-945d-eddc440aa0b2")), GenerateAssetId(stc))
	assert.Equal(crypto.NewHash([]byte("c99a3779-93df-404d-945d-eddc440aa0b2")), StarcoinChainId)
	assert.Equal(crypto.NewHash([]byte(StarcoinChainBase)), StarcoinChainId)
}
