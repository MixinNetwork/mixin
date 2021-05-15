package stellar

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	xlm := "56e63c06-b506-4ec5-885a-4a5ac17b83c1"
	tx := "fa01f7b2391eac01662316f1611be34611c28bd4746026f69b89ad86e9b9f581"
	addrMain := "GD77JOIFC622O5HXU446VIKGR5A5HMSTAUKO2FSN5CIVWPHXDBGIAG7Y"

	assert.Nil(VerifyAssetKey(xlm))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(xlm)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(xlm))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToLower(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(xlm))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("56e63c06-b506-4ec5-885a-4a5ac17b83c1")), GenerateAssetId(xlm))
	assert.Equal(crypto.NewHash([]byte("56e63c06-b506-4ec5-885a-4a5ac17b83c1")), StellarChainId)
	assert.Equal(crypto.NewHash([]byte(StellarChainBase)), StellarChainId)
}
