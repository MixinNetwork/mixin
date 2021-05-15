package kusama

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	ksm := "9d29e4f6-d67c-4c4b-9525-604b04afbe9f"
	tx := "0x961c4418df4afdbc2dcca2a146e01eadc8a56f76515c523ee1bda55d46e4b3e0"
	addrMain := "F4xQKRUagnSGjFqafyhajLs94e7Vvzvr8ebwYJceKpr8R7T"

	assert.Nil(VerifyAssetKey(ksm))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(ksm)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(ksm))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(ksm))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("9d29e4f6-d67c-4c4b-9525-604b04afbe9f")), GenerateAssetId(ksm))
	assert.Equal(crypto.NewHash([]byte("9d29e4f6-d67c-4c4b-9525-604b04afbe9f")), KusamaChainId)
	assert.Equal(crypto.NewHash([]byte(KusamaChainBase)), KusamaChainId)
}
