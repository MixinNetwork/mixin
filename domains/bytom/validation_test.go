package bytom

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	btm := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	tx := "ee7bafb9233adae99d88da66a3fbeb6d40b1cc9dbad0f3d2abb3f6a69b28ae7b"
	addrMain := "bn1qc2dzlkky2cvvvw79u9n858xak5hcfr47c489qe"

	assert.Nil(VerifyAssetKey(btm))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(btm)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(btm))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(btm))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("71a0e8b5-a289-4845-b661-2b70ff9968aa")), GenerateAssetId(btm))
	assert.Equal(crypto.NewHash([]byte("71a0e8b5-a289-4845-b661-2b70ff9968aa")), BytomChainId)
	assert.Equal(crypto.NewHash([]byte(BytomChainBase)), BytomChainId)
}
