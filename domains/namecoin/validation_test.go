package namecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	nmc := "f8b77dc0-46fd-4ea1-9821-587342475869"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "NCjrV4CWpSr73mfYADbiujetMB3F3VrDWc"

	require.Nil(VerifyAssetKey(nmc))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(nmc)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(nmc))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(nmc))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("f8b77dc0-46fd-4ea1-9821-587342475869")), GenerateAssetId(nmc))
	require.Equal(crypto.NewHash([]byte("f8b77dc0-46fd-4ea1-9821-587342475869")), NamecoinChainId)
	require.Equal(crypto.NewHash([]byte(NamecoinChainBase)), NamecoinChainId)
}
