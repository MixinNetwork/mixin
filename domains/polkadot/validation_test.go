package polkadot

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	sc := "54c61a72-b982-4034-a556-0d99e3c21e39"
	tx := "0x4e19029390b67d1c5b3b589939e268cd22d9495ecd8375d3d7143a6946e4a359"
	addrMain := "16NyUzZDzKbQe4zY6D9PSLhYH6CeeQXP1BdMK9DgN4o29MBx"

	assert.Nil(VerifyAssetKey(sc))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(sc)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(sc))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(sc))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("54c61a72-b982-4034-a556-0d99e3c21e39")), GenerateAssetId(sc))
	assert.Equal(crypto.NewHash([]byte("54c61a72-b982-4034-a556-0d99e3c21e39")), PolkadotChainId)
	assert.Equal(crypto.NewHash([]byte(PolkadotChainBase)), PolkadotChainId)
}
