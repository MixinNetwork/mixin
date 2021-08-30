package akash

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	assetKey := "uakt"
	tx := "e2adef1954f5eee1bd9f4defa7080b6b61a8b9de650120ba9722ab8674e6f38a"
	addrMain := "akash1f9su26yet620lndeyzmun5x5sk6wfslv4xxtgt"

	assert.Nil(VerifyAssetKey(assetKey))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(assetKey)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(assetKey))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(assetKey))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("9c612618-ca59-4583-af34-be9482f5002d")), GenerateAssetId(assetKey))
	assert.Equal(crypto.NewHash([]byte("9c612618-ca59-4583-af34-be9482f5002d")), AkashChainId)
	assert.Equal(crypto.NewHash([]byte(AkashChainBase)), AkashChainId)
}
