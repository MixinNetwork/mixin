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
	tx := "b7b3746bcc056761727292f311c171a09d2800dc5296db8265f2cc556f38677e"
	addrMain := "d3e13d4777e22367532053190b6c6ccf57444a61337e996242b1abfb52cf92c8"

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
