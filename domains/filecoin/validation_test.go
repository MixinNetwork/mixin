package filecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	fil := "08285081-e1d8-4be6-9edc-e203afa932da"
	tx := "bafy2bzaceaqr65fthy3z4wn2rmo7ani75sekd5kwsg3pkrzznynopbgnovtkc"
	addrMain := "f1egh23o5qy2ibkqwawqyjague4urpxiyf672l6zi"

	assert.Nil(VerifyAssetKey(fil))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(fil)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(fil))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(fil))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("08285081-e1d8-4be6-9edc-e203afa932da")), GenerateAssetId(fil))
	assert.Equal(crypto.NewHash([]byte("08285081-e1d8-4be6-9edc-e203afa932da")), FilecoinChainId)
	assert.Equal(crypto.NewHash([]byte(FilecoinChainBase)), FilecoinChainId)
}
