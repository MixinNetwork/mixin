package binance

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	bnb := "BNB"
	tx := "752b23fa8585f2516022a481c6c57f42f355cbb79560e7f26520ddb027ecc48f"
	addrMain := "bnb1rmc2xnpgx48hfq5jr8hqzh02ewl26dz5k0vfu7"

	assert.Nil(VerifyAssetKey(bnb))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToLower(bnb)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(bnb))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(bnb))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("17f78d7c-ed96-40ff-980c-5dc62fecbc85")), GenerateAssetId(bnb))
	assert.Equal(crypto.NewHash([]byte("17f78d7c-ed96-40ff-980c-5dc62fecbc85")), BinanceChainId)
	assert.Equal(crypto.NewHash([]byte(BinanceChainBase)), BinanceChainId)
	assert.Equal(crypto.NewHash([]byte("f312d6a7-1b4d-34c0-bf84-75e657a3fcf3")), GenerateAssetId("BUSD-BD1"))
}
