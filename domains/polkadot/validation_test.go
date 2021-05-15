package polkadot

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	dot := "54c61a72-b982-4034-a556-0d99e3c21e39"
	tx := "0x69cb313180b82f8d98314fc57c09905acc82282df3d068091e2344ea35a85c5a"
	addrMain := "13eM4Bgw55j93P7tiozfSjCkr55imbbiyso9MTG6YiQLaZSt"

	assert.Nil(VerifyAssetKey(dot))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(dot)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(dot))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(dot))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("54c61a72-b982-4034-a556-0d99e3c21e39")), GenerateAssetId(dot))
	assert.Equal(crypto.NewHash([]byte("54c61a72-b982-4034-a556-0d99e3c21e39")), PolkadotChainId)
	assert.Equal(crypto.NewHash([]byte(PolkadotChainBase)), PolkadotChainId)
}
