package akash

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	assetKey := "uakt"
	tx := "e2adef1954f5eee1bd9f4defa7080b6b61a8b9de650120ba9722ab8674e6f38a"
	addrMain := "akash1f9su26yet620lndeyzmun5x5sk6wfslv4xxtgt"

	require.Nil(VerifyAssetKey(assetKey))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(assetKey)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(assetKey))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(assetKey))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("9c612618-ca59-4583-af34-be9482f5002d")), GenerateAssetId(assetKey))
	require.Equal(crypto.NewHash([]byte("9c612618-ca59-4583-af34-be9482f5002d")), AkashChainId)
	require.Equal(crypto.NewHash([]byte(AkashChainBase)), AkashChainId)
}
