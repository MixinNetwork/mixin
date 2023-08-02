package dash

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	dash := "6472e7e3-75fd-48b6-b1dc-28d294ee1476"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "XksUwk1GETexCpP6Wbrdswd3TfWRSckUAn"

	require.Nil(VerifyAssetKey(dash))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(dash)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(dash))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(dash))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("6472e7e3-75fd-48b6-b1dc-28d294ee1476")), GenerateAssetId(dash))
	require.Equal(crypto.NewHash([]byte("6472e7e3-75fd-48b6-b1dc-28d294ee1476")), DashChainId)
	require.Equal(crypto.NewHash([]byte(DashChainBase)), DashChainId)
}
