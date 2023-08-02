package arweave

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	ar := "882eb041-64ea-465f-a4da-817bd3020f52"
	tx := "5_-HdBC72aXmM0b9NmHbDBZdcvwdhcNfj7Rqts9YtQE"
	addrMain := "9dE4RwCxwElyc0YDfzgYmeMZhyDuhfnMmq8N95J8pIg"

	require.Nil(VerifyAssetKey(ar))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(ar)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(ar))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(ar))
	require.NotNil(VerifyTransactionHash("0x" + tx))

	require.Equal(crypto.NewHash([]byte("882eb041-64ea-465f-a4da-817bd3020f52")), GenerateAssetId(ar))
	require.Equal(crypto.NewHash([]byte("882eb041-64ea-465f-a4da-817bd3020f52")), ArweaveChainId)
	require.Equal(crypto.NewHash([]byte(ArweaveChainBase)), ArweaveChainId)
}
