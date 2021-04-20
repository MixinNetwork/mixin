package handshake

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	hns := "13036886-6b83-4ced-8d44-9f69151587bf"
	tx := "8c30eece44c9b4f4314f06ec5eedc7486e83ae76159ea81a0ee7aac2f16bbf0b"
	addrMain := "hs1qsh9v47p3k75lk9js8dptdd4qcy3n0scd33lm4j"

	assert.Nil(VerifyAssetKey(hns))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(hns)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(hns))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(hns))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("13036886-6b83-4ced-8d44-9f69151587bf")), GenerateAssetId(hns))
	assert.Equal(crypto.NewHash([]byte("13036886-6b83-4ced-8d44-9f69151587bf")), HandshakenChainId)
	assert.Equal(crypto.NewHash([]byte(HandshakenChainBase)), HandshakenChainId)
}
