package tezos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	xtz := "5649ca42-eb5f-4c0e-ae28-d9a4e77eded3"
	tx := "oodYJNMcvbi1uyVVE6c14LWU64mwtTw4n444L8rwsGmg6oT5kuB"
	addrMain := "tz1LNGzjz8H9juHNrHLKbZ1fm7un3KJpxsFY"

	assert.Nil(VerifyAssetKey(xtz))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(xtz)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(xtz))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToLower(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(xtz))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("5649ca42-eb5f-4c0e-ae28-d9a4e77eded3")), GenerateAssetId(xtz))
	assert.Equal(crypto.NewHash([]byte("5649ca42-eb5f-4c0e-ae28-d9a4e77eded3")), TezosChainId)
	assert.Equal(crypto.NewHash([]byte(TezosChainBase)), TezosChainId)
}
