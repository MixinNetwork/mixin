package mvm

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	xin := "0x034a771797a1c8694bc33e1aa89f51d1f828e5a4"
	tx := "0xc5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"

	xinFormat, _ := formatAddress(xin)
	assert.Equal("0x034A771797a1C8694Bc33E1AA89f51d1f828e5A4", xinFormat)

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

	assert.Equal(crypto.NewHash([]byte("9437ebfa-873a-3032-8295-eedb8a3c86c7")), GenerateAssetId(xin))
	assert.Equal(crypto.NewHash([]byte("a0ffd769-5850-4b48-9651-d2ae44a3e64d")), GenerateAssetId("0x0000000000000000000000000000000000000000"))
	assert.Equal(crypto.NewHash([]byte("a0ffd769-5850-4b48-9651-d2ae44a3e64d")), MVMChainId)
}
