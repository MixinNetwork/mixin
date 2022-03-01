package near

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	near := "d6ac94f7-c932-4e11-97dd-617867f0669e"
	tx := "8Z87eXBbFQN1b91UVVHsASeFPvucCZmmG9oae6wZV6uN"
	addrMain := "d6b52637bf0e03a253a634a64705580ed0d2d58479613a0aa13c4342db172323"
	addrInvalid := "d6b52637bf0e03a253a634a64705580ed0d2d58479613a0aa13c4342db172321"
	addrMain2 := "app.nearcrowd.near"

	assert.Nil(VerifyAssetKey(near))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(near)))

	assert.Nil(VerifyAddress(addrMain))
	assert.Nil(VerifyAddress(addrMain2))
	assert.NotNil(VerifyAddress(near))
	assert.NotNil(VerifyAddress(addrInvalid))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(near))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("d6ac94f7-c932-4e11-97dd-617867f0669e")), GenerateAssetId(near))
	assert.Equal(crypto.NewHash([]byte("d6ac94f7-c932-4e11-97dd-617867f0669e")), NearChainId)
	assert.Equal(crypto.NewHash([]byte(NearChainBase)), NearChainId)
}
