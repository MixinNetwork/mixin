package terra

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	assetKey := "uluna"
	assetUsdKey := "uusd"
	anc := "terra14z56l0fp2lsf86zy3hty2z47ezkhnthtr9yq76"
	tx := "99a2a8bcd5da27cc910649f03259c8446a76d6345973be3922026c3dee9bcb1f"
	addrMain := "terra158n5uhvygpz5ttunfuaqh0l2ly5vhl72fy7d8q"

	assert.Nil(VerifyAssetKey(assetKey))
	assert.Nil(VerifyAssetKey(assetUsdKey))
	assert.Nil(VerifyAssetKey(anc))
	assert.Nil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(tx))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(assetKey))
	assert.NotNil(VerifyAddress(assetUsdKey))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(assetKey))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("eb5bb26d-bfda-4e63-bf1d-a462b78343b7")), GenerateAssetId(assetKey))
	assert.Equal(crypto.NewHash([]byte("cd54d4a2-6b64-3fe2-a1bc-16bb26deb2a3")), GenerateAssetId(anc))
	assert.Equal(crypto.NewHash([]byte(TerraChainBase)), TerraChainId)
}
