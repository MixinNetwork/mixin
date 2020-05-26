// +build ed25519 !custom_alg

package ethereum

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/crypto/ed25519"
	"github.com/stretchr/testify/assert"
)

func init() {
	ed25519.Load()
}

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	xin := "0xa974c709cfb4566686553a20790685a47aceaa33"
	usdt := "0xdac17f958d2ee523a2206206994597c13d831ec7"
	tx := "0xc5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"

	xinFormat, _ := formatAddress(xin)
	usdtFormat, _ := formatAddress(usdt)
	assert.Equal("0xA974c709cFb4566686553a20790685A47acEAA33", xinFormat)
	assert.Equal("0xdAC17F958D2ee523a2206206994597C13D831ec7", usdtFormat)

	assert.Nil(VerifyAssetKey(xin))
	assert.Nil(VerifyAssetKey(usdt))
	assert.NotNil(VerifyAssetKey(xinFormat))
	assert.NotNil(VerifyAssetKey(usdtFormat))
	assert.NotNil(VerifyAssetKey(xin[2:]))
	assert.NotNil(VerifyAssetKey(usdt[2:]))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(xin)))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(usdt)))

	assert.Nil(VerifyAddress(xinFormat))
	assert.Nil(VerifyAddress(usdtFormat))
	assert.NotNil(VerifyAddress(xin))
	assert.NotNil(VerifyAddress(usdt))
	assert.NotNil(VerifyAddress(xin[2:]))
	assert.NotNil(VerifyAddress(usdt[2:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(xin)))
	assert.NotNil(VerifyAddress(strings.ToUpper(usdt)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(xin))
	assert.NotNil(VerifyTransactionHash(tx[2:]))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8")), GenerateAssetId(xin))
	assert.Equal(crypto.NewHash([]byte("4d8c508b-91c5-375b-92b0-ee702ed2dac5")), GenerateAssetId(usdt))
	assert.Equal(crypto.NewHash([]byte("43d61dcd-e413-450d-80b8-101d5e903357")), GenerateAssetId("0x0000000000000000000000000000000000000000"))
	assert.Equal(crypto.NewHash([]byte("43d61dcd-e413-450d-80b8-101d5e903357")), EthereumChainId)
}
