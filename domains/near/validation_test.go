package near

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	near := "d6ac94f7-c932-4e11-97dd-617867f0669e"
	tx := "8Z87eXBbFQN1b91UVVHsASeFPvucCZmmG9oae6wZV6uN"
	addrMain := "d6b52637bf0e03a253a634a64705580ed0d2d58479613a0aa13c4342db172323"
	addrInvalid := "d6b52637bf0e03a253a634a64705580ed0d2d58479613a0aa13c4342db172321"

	require.Nil(VerifyAssetKey(near))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(near)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(near))
	require.NotNil(VerifyAddress(addrInvalid))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(near))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("d6ac94f7-c932-4e11-97dd-617867f0669e")), GenerateAssetId(near))
	require.Equal(crypto.NewHash([]byte("d6ac94f7-c932-4e11-97dd-617867f0669e")), NearChainId)
	require.Equal(crypto.NewHash([]byte(NearChainBase)), NearChainId)
}
