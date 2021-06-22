package dfinity

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	icp := "d5db6f39-fe50-4633-8edc-36e2f3e117e4"
	tx := "8614fec5bc43d40fbc252ac3b042b7a01d622338e073d790d2da501cab845a8c"
	addrMain := "449ce7ad1298e2ed2781ed379aba25efc2748d14c60ede190ad7621724b9e8b2"

	assert.Nil(VerifyAssetKey(icp))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(icp)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(icp))
	assert.NotNil(VerifyAddress(addrMain[1:]))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(icp))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("d5db6f39-fe50-4633-8edc-36e2f3e117e4")), GenerateAssetId(icp))
	assert.Equal(crypto.NewHash([]byte("d5db6f39-fe50-4633-8edc-36e2f3e117e4")), DfinityChainId)
	assert.Equal(crypto.NewHash([]byte(DfinityChainBase)), DfinityChainId)
}
