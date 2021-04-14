package decred

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	dcr := "8f5caf2a-283d-4c85-832a-91e83bbf290b"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "DsoBw7Xa2dh1pRYcmFC3npi4Mh4ZydbMzUH"

	assert.Nil(VerifyAssetKey(dcr))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(dcr)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(dcr))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(dcr))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("8f5caf2a-283d-4c85-832a-91e83bbf290b")), GenerateAssetId(dcr))
	assert.Equal(crypto.NewHash([]byte("8f5caf2a-283d-4c85-832a-91e83bbf290b")), DecredChainId)
	assert.Equal(crypto.NewHash([]byte(DecredChainBase)), DecredChainId)
}
