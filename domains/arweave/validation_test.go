package arweave

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	ar := "882eb041-64ea-465f-a4da-817bd3020f52"
	tx := "5_-HdBC72aXmM0b9NmHbDBZdcvwdhcNfj7Rqts9YtQE"
	addrMain := "9dE4RwCxwElyc0YDfzgYmeMZhyDuhfnMmq8N95J8pIg"

	assert.Nil(VerifyAssetKey(ar))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(ar)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(ar))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(ar))
	assert.NotNil(VerifyTransactionHash("0x" + tx))

	assert.Equal(crypto.NewHash([]byte("882eb041-64ea-465f-a4da-817bd3020f52")), GenerateAssetId(ar))
	assert.Equal(crypto.NewHash([]byte("882eb041-64ea-465f-a4da-817bd3020f52")), ArweaveChainId)
	assert.Equal(crypto.NewHash([]byte(ArweaveChainBase)), ArweaveChainId)
}
