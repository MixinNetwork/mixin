package starcoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	stc := "0x00000000000000000000000000000001::STC::STC"
	tx := "0xa1dc5ccb8c7ebcb557c3514ed851a23e07c3311faf20b95cba8a0f5d1648ff11"
	addrMain := "0x7a86a44a5c5ed4827402dd09db4a2353"

	require.Nil(VerifyAssetKey(stc))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(stc)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(stc))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(stc))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("c99a3779-93df-404d-945d-eddc440aa0b2")), GenerateAssetId(stc))
	require.Equal(crypto.NewHash([]byte("c99a3779-93df-404d-945d-eddc440aa0b2")), StarcoinChainId)
	require.Equal(crypto.NewHash([]byte(StarcoinChainBase)), StarcoinChainId)
}
