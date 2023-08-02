package avalanche

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	assetKey := "FvwEAhmxKfeiG8SnEvq42hc6whRyY3EFYAvebMqDNDGCgxN5Z"
	tx := "Sv3wdQnUfh7A9zGzppHxn7ehjzkFR79MMnQdx2CUWdRc3eSNN"
	addrMain := "X-avax1emj30lmw3mcdgnmzl2plrmmvahln9mnmfzw2d5"

	require.Nil(VerifyAssetKey(assetKey))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(assetKey)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(assetKey))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))

	require.Equal(crypto.NewHash([]byte("cbc77539-0a20-4666-8c8a-4ded62b36f0a")), GenerateAssetId(assetKey))
	require.Equal(crypto.NewHash([]byte("cbc77539-0a20-4666-8c8a-4ded62b36f0a")), AvalancheChainId)
	require.Equal(crypto.NewHash([]byte(AvalancheChainBase)), AvalancheChainId)
}
