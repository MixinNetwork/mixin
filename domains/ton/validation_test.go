package ton

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	ton := "ef660437-d915-4e27-ad3f-632bfb6ba0ee"
	tx := "G1x5VxX34d4osWouOWJY95T-s_nKwyP2TpT-jZVr2M0="
	addrMain := "EQDg3BjFRYIoI63laXTeg8tSbLza_zx5-hA6lg6hatdm9m_L"
	addrMain2 := "UQBEWdaTIQ76d-U5dBA65BmPqirv299s2Q4LsqjTmoh6Nvax"

	require.Nil(VerifyAssetKey(ton))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(ton)))

	require.Nil(VerifyAddress(addrMain))
	require.Nil(VerifyAddress(addrMain2))
	require.NotNil(VerifyAddress(ton))
	require.NotNil(VerifyAddress(addrMain[1:]))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(ton))
	require.NotNil(VerifyTransactionHash("0x" + tx))

	require.Equal(crypto.NewHash([]byte("ef660437-d915-4e27-ad3f-632bfb6ba0ee")), GenerateAssetId(TonAssetKey))
	require.Equal(crypto.NewHash([]byte("ef660437-d915-4e27-ad3f-632bfb6ba0ee")), TonChainId)
	require.Equal(crypto.NewHash([]byte(TonChainBase)), TonChainId)
}
