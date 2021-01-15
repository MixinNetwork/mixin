package horizen

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	zen := "a2c5d22b-62a2-4c13-b3f0-013290dbac60"
	tx := "8c30eece44c9b4f4314f06ec5eedc7486e83ae76159ea81a0ee7aac2f16bbf0b"
	addrMain := "zszpcLB6C5B8QvfDbF2dYWXsrpac5DL9WRk"

	assert.Nil(VerifyAssetKey(zen))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(zen)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(zen))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(zen))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("a2c5d22b-62a2-4c13-b3f0-013290dbac60")), GenerateAssetId(zen))
	assert.Equal(crypto.NewHash([]byte("a2c5d22b-62a2-4c13-b3f0-013290dbac60")), HorizenChainId)
	assert.Equal(crypto.NewHash([]byte(HorizenChainBase)), HorizenChainId)
}
