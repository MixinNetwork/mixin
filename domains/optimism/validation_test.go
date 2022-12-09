package optimism

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	xin := "0xa974c709cfb4566686553a20790685a47aceaa33"
	tx := "0xc5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"

	xinFormat, _ := formatAddress(xin)
	assert.Equal("0xA974c709cFb4566686553a20790685A47acEAA33", xinFormat)

	assert.Nil(VerifyAssetKey("0x0000000000000000000000000000000000000000"))
	assert.Nil(VerifyAssetKey(xin))
	assert.NotNil(VerifyAssetKey(xinFormat))
	assert.NotNil(VerifyAssetKey(xin[2:]))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(xin)))

	assert.Nil(VerifyAddress(xinFormat))
	assert.NotNil(VerifyAddress(xin))
	assert.NotNil(VerifyAddress(xin[2:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(xin)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(xin))
	assert.NotNil(VerifyTransactionHash(tx[2:]))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("62d5b01f-24ee-4c96-8214-8e04981d05f2")), GenerateAssetId("0x0000000000000000000000000000000000000000"))
	assert.Equal(crypto.NewHash([]byte("62d5b01f-24ee-4c96-8214-8e04981d05f2")), OptimismChainId)
}
