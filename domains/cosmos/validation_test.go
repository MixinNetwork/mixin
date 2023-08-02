package cosmos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	atom := "uatom"
	tx := "c9698260bab4095df25a228a3d855918de38a9e0c57d7a137de18b4c141f26ee"
	addrMain := "cosmos14xwf5zcf0qk2t8vuqtr0zv9yt9g85dust0u68d"

	require.Nil(VerifyAssetKey(atom))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(atom)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(atom))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(atom))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("7397e9f1-4e42-4dc8-8a3b-171daaadd436")), GenerateAssetId(atom))
	require.Equal(crypto.NewHash([]byte("7397e9f1-4e42-4dc8-8a3b-171daaadd436")), CosmosChainId)
	require.Equal(crypto.NewHash([]byte(CosmosChainBase)), CosmosChainId)
}
