package ripple

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	xrp := "23dfb5a5-5d7b-48b6-905f-3970e3176e27"
	tx := "564D15A614B47A01D9F3AD08EC298ED8D7A7ECC98F4D64627D4D6A559668DBC8"
	addrMain := "rK6Vezau2D1FDUhFs1me35H3xod8UKc1Go"

	assert.Nil(VerifyAssetKey(xrp))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(xrp)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(xrp))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(xrp))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToLower(tx)))

	assert.Equal(crypto.NewHash([]byte("23dfb5a5-5d7b-48b6-905f-3970e3176e27")), GenerateAssetId(xrp))
	assert.Equal(crypto.NewHash([]byte("23dfb5a5-5d7b-48b6-905f-3970e3176e27")), RippleChainId)
	assert.Equal(crypto.NewHash([]byte(RippleChainBase)), RippleChainId)
}
