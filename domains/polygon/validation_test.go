package polygon

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	tusd := "0x2e1ad108ff1d8c782fcbbb89aad783ac49586756"
	usdc := "0x2791bca1f2de4661ed88a30c99a7a9449aa84174"
	tx := "0xc5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"

	tusdFormat, _ := formatAddress(tusd)
	usdcFormat, _ := formatAddress(usdc)
	assert.Equal("0x2e1AD108fF1D8C782fcBbB89AAd783aC49586756", tusdFormat)
	assert.Equal("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174", usdcFormat)

	assert.Nil(VerifyAssetKey(tusd))
	assert.Nil(VerifyAssetKey(usdc))
	assert.NotNil(VerifyAssetKey(tusdFormat))
	assert.NotNil(VerifyAssetKey(usdcFormat))
	assert.NotNil(VerifyAssetKey(tusd[2:]))
	assert.NotNil(VerifyAssetKey(usdc[2:]))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(tusd)))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(usdc)))

	assert.Nil(VerifyAddress(tusdFormat))
	assert.Nil(VerifyAddress(usdcFormat))
	assert.NotNil(VerifyAddress(tusd))
	assert.NotNil(VerifyAddress(usdc))
	assert.NotNil(VerifyAddress(tusd[2:]))
	assert.NotNil(VerifyAddress(usdc[2:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(tusd)))
	assert.NotNil(VerifyAddress(strings.ToUpper(usdc)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(tusd))
	assert.NotNil(VerifyTransactionHash(tx[2:]))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("9189a528-c3a5-36cb-8e08-feb81e7cb9cb")), GenerateAssetId(tusd))
	assert.Equal(crypto.NewHash([]byte("80b65786-7c75-3523-bc03-fb25378eae41")), GenerateAssetId(usdc))
	assert.Equal(crypto.NewHash([]byte("b7938396-3f94-4e0a-9179-d3440718156f")), GenerateAssetId("0x0000000000000000000000000000000000001010"))
	assert.Equal(crypto.NewHash([]byte("b7938396-3f94-4e0a-9179-d3440718156f")), PolygonChainId)
}
