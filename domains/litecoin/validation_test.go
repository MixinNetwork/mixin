package litecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	ltc := "76c802a2-7c88-447f-a93e-c29c9e5dd9c8"
	tx := "b17c33501a8f52918f9c80723420a5f4fd39be2de117ec8343239d3a98b467c1"
	addrMain := "LfmssDyX6iZvbVqHv6t9P6JWXia2JG7mdb"

	assert.Nil(VerifyAssetKey(ltc))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(ltc)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(ltc))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(ltc))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("76c802a2-7c88-447f-a93e-c29c9e5dd9c8")), GenerateAssetId(ltc))
	assert.Equal(crypto.NewHash([]byte("76c802a2-7c88-447f-a93e-c29c9e5dd9c8")), LitecoinChainId)
	assert.Equal(crypto.NewHash([]byte(LitecoinChainBase)), LitecoinChainId)
}
