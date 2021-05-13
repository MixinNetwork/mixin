package ravencoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	rvn := "6877d485-6b64-4225-8d7e-7333393cb243"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "RE9x1e1u6nXiaMq1eFstcK8whQ4NhGz1mP"

	assert.Nil(VerifyAssetKey(rvn))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(rvn)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(rvn))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(rvn))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("6877d485-6b64-4225-8d7e-7333393cb243")), GenerateAssetId(rvn))
	assert.Equal(crypto.NewHash([]byte("6877d485-6b64-4225-8d7e-7333393cb243")), RavencoinChainId)
	assert.Equal(crypto.NewHash([]byte(RavencoinChainBase)), RavencoinChainId)
}
