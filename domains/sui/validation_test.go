package sui

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	sui := "0x2::sui::SUI"
	tx := "HYoT2TjuDWTBkSfbUwCGqyUoeH9L5dA1j3Ydg1nsSeRt"
	addrMain := "0xc7cfe9ed707f3e326e69ebd68295ca0d428fd09b4e59c06ef1f35bdb70bbfec5"

	assert.Nil(VerifyAssetKey(sui))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(sui)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(sui))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(sui))
	assert.NotNil(VerifyTransactionHash("0x" + tx))

	assert.Equal(crypto.NewHash([]byte("b1ad4729-2c39-4e7e-8bd6-c63c21941a0e")), GenerateAssetId(SuiAssetKey))
	assert.Equal(crypto.NewHash([]byte("b1ad4729-2c39-4e7e-8bd6-c63c21941a0e")), SuiChainId)
	assert.Equal(crypto.NewHash([]byte(SuiChainBase)), SuiChainId)
}
