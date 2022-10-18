package aptos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	apt := "0x1::aptos_coin::AptosCoin"
	tx := "0xef850268f585c82a02deaedcfc3cc9962668a02209974b77adb1a7c5f0b974d5"
	addrMain := "0x89fa1b72e65fab3da9a42dfe28047c658a5f1ab8857daaf9621b62156baec9f4"

	assert.Nil(VerifyAssetKey(apt))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(apt)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(apt))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(apt))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("d2c1c7e1-a1a9-4f88-b282-d93b0a08b42b")), GenerateAssetId(AptosAssetKey))
	assert.Equal(crypto.NewHash([]byte("d2c1c7e1-a1a9-4f88-b282-d93b0a08b42b")), AptosChainId)
	assert.Equal(crypto.NewHash([]byte(AptosChainBase)), AptosChainId)
}
