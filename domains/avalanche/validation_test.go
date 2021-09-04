package avalanche

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	assetKey := "FvwEAhmxKfeiG8SnEvq42hc6whRyY3EFYAvebMqDNDGCgxN5Z"
	tx := "Sv3wdQnUfh7A9zGzppHxn7ehjzkFR79MMnQdx2CUWdRc3eSNN"
	addrMain := "X-avax1emj30lmw3mcdgnmzl2plrmmvahln9mnmfzw2d5"

	assert.Nil(VerifyAssetKey(assetKey))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(assetKey)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(assetKey))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))

	assert.Equal(crypto.NewHash([]byte("cbc77539-0a20-4666-8c8a-4ded62b36f0a")), GenerateAssetId(assetKey))
	assert.Equal(crypto.NewHash([]byte("cbc77539-0a20-4666-8c8a-4ded62b36f0a")), AvalancheChainId)
	assert.Equal(crypto.NewHash([]byte(AvalancheChainBase)), AvalancheChainId)
}
