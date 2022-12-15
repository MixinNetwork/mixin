package ton

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	ton := "ef660437-d915-4e27-ad3f-632bfb6ba0ee"
	tx := "G1x5VxX34d4osWouOWJY95T-s_nKwyP2TpT-jZVr2M0="
	addrMain := "EQDg3BjFRYIoI63laXTeg8tSbLza_zx5-hA6lg6hatdm9m_L"

	assert.Nil(VerifyAssetKey(ton))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(ton)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(ton))
	assert.NotNil(VerifyAddress(addrMain[1:]))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(ton))
	assert.NotNil(VerifyTransactionHash("0x" + tx))

	assert.Equal(crypto.NewHash([]byte("ef660437-d915-4e27-ad3f-632bfb6ba0ee")), GenerateAssetId(TonAssetKey))
	assert.Equal(crypto.NewHash([]byte("ef660437-d915-4e27-ad3f-632bfb6ba0ee")), TonChainId)
	assert.Equal(crypto.NewHash([]byte(TonChainBase)), TonChainId)
}
