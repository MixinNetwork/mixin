package mobilecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	mob := "eea900a8-b327-488c-8d8d-1428702fe240"
	tx := "40c7e63c8cd2ddb1e65ffd3531e47739ed78cdcfef9cfd5cb6916f3c50d19c16"
	addr := "G57w8Br44AYd6aEKfagTyLFvt4tTLhDdzGsX6PbYwfumwpjc1htSpWfoey2FLYNKMJA28q8YyqYb83dh66A7BTVA4XNZzXsNNUDv1nTmaw"

	assert.Nil(VerifyAssetKey(mob))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addr))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(mob)))

	assert.Nil(VerifyAddress(addr))
	assert.NotNil(VerifyAddress("G57w8Br44AYd6aEKfagTy"))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(mob))
	assert.NotNil(VerifyTransactionHash(addr))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("eea900a8-b327-488c-8d8d-1428702fe240")), GenerateAssetId(mob))
	assert.Equal(crypto.NewHash([]byte("eea900a8-b327-488c-8d8d-1428702fe240")), MobileCoinChainId)
	assert.Equal(crypto.NewHash([]byte(MobileCoinChainBase)), MobileCoinChainId)
}
