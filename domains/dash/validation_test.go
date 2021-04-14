package dash

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	dash := "6472e7e3-75fd-48b6-b1dc-28d294ee1476"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "XksUwk1GETexCpP6Wbrdswd3TfWRSckUAn"

	assert.Nil(VerifyAssetKey(dash))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(dash)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(dash))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(dash))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("6472e7e3-75fd-48b6-b1dc-28d294ee1476")), GenerateAssetId(dash))
	assert.Equal(crypto.NewHash([]byte("6472e7e3-75fd-48b6-b1dc-28d294ee1476")), DashChainId)
	assert.Equal(crypto.NewHash([]byte(DashChainBase)), DashChainId)
}
