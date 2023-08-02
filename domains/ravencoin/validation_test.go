package ravencoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	rvn := "6877d485-6b64-4225-8d7e-7333393cb243"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "RE9x1e1u6nXiaMq1eFstcK8whQ4NhGz1mP"

	require.Nil(VerifyAssetKey(rvn))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(rvn)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(rvn))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(rvn))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("6877d485-6b64-4225-8d7e-7333393cb243")), GenerateAssetId(rvn))
	require.Equal(crypto.NewHash([]byte("6877d485-6b64-4225-8d7e-7333393cb243")), RavencoinChainId)
	require.Equal(crypto.NewHash([]byte(RavencoinChainBase)), RavencoinChainId)
}
