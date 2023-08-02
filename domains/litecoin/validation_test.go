package litecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	ltc := "76c802a2-7c88-447f-a93e-c29c9e5dd9c8"
	tx := "b17c33501a8f52918f9c80723420a5f4fd39be2de117ec8343239d3a98b467c1"
	addrMain := "LcDrhX7NCmoRj58abHjAzfNCvk7jHxARsm"
	addrLegacy := "37EstF3KLGpXFLGXGZCURmdSZzjCVMbekC"
	addrSegwit := "ltc1q3v6al5dh59ej5vhut87595460mflj55xpe82jhplfa57p2yvfrusaecf5l"

	require.Nil(VerifyAssetKey(ltc))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(ltc)))

	require.Nil(VerifyAddress(addrMain))
	require.Nil(VerifyAddress(addrLegacy))
	require.Nil(VerifyAddress(addrSegwit))
	require.NotNil(VerifyAddress(ltc))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(ltc))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("76c802a2-7c88-447f-a93e-c29c9e5dd9c8")), GenerateAssetId(ltc))
	require.Equal(crypto.NewHash([]byte("76c802a2-7c88-447f-a93e-c29c9e5dd9c8")), LitecoinChainId)
	require.Equal(crypto.NewHash([]byte(LitecoinChainBase)), LitecoinChainId)
}
