package solana

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	sol := "11111111111111111111111111111111"
	tx := "66xB9R354Bre7RpGEbQ7CPFxDioyQ1ejrMJhSa2jW1DQzhhoaS3SUiEmQxx3nzqHv1yUW7kkSiVcTtL7vdn34o6o"
	addrMain := "GuscxHWgjxoMTokbW5bmt54WnHAVEtyE3RCVXgxdZjnG"

	assert.Nil(VerifyAssetKey(sol))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(sol))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(sol))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("64692c23-8971-4cf4-84a7-4dd1271dd887")), GenerateAssetId(sol))
	assert.Equal(crypto.NewHash([]byte("64692c23-8971-4cf4-84a7-4dd1271dd887")), SolanaChainId)
	assert.Equal(crypto.NewHash([]byte(SolanaChainBase)), SolanaChainId)
}
