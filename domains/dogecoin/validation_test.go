package dogecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	doge := "6770a1e5-6086-44d5-b60f-545f9d9e8ffd"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "DANHz6EQVoWyZ9rER56DwTXHWUxfkv9k2o"

	assert.Nil(VerifyAssetKey(doge))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(doge)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(doge))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(doge))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("6770a1e5-6086-44d5-b60f-545f9d9e8ffd")), GenerateAssetId(doge))
	assert.Equal(crypto.NewHash([]byte("6770a1e5-6086-44d5-b60f-545f9d9e8ffd")), DogecoinChainId)
	assert.Equal(crypto.NewHash([]byte(DogecoinChainBase)), DogecoinChainId)
}
