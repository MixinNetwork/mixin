package xdc

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	xin := "xdc8f9920283470f52128bf11b0c14e798be704fd15"
	tx := "0x84326a8499614f7236ff22fd6d6aa473649c1704404ca86f37953394f4659127"

	xinFormat, _ := formatAddress(xin)
	assert.Equal("xdc8f9920283470F52128bF11B0c14E798bE704fD15", xinFormat)

	assert.Nil(VerifyAssetKey("xdc0000000000000000000000000000000000000000"))
	assert.Nil(VerifyAssetKey(xin))
	assert.NotNil(VerifyAssetKey(xinFormat))
	assert.NotNil(VerifyAssetKey(xin[3:]))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(xin)))

	assert.Nil(VerifyAddress(xinFormat))
	assert.NotNil(VerifyAddress(xin))
	assert.NotNil(VerifyAddress(xin[3:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(xin)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(xin))
	assert.NotNil(VerifyTransactionHash(tx[2:]))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("b12bb04a-1cea-401c-a086-0be61f544889")), GenerateAssetId("xdc0000000000000000000000000000000000000000"))
	assert.Equal(crypto.NewHash([]byte("b12bb04a-1cea-401c-a086-0be61f544889")), XDCChainId)
}
