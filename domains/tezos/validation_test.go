package tezos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	xtz := "5649ca42-eb5f-4c0e-ae28-d9a4e77eded3"
	tx := "oodYJNMcvbi1uyVVE6c14LWU64mwtTw4n444L8rwsGmg6oT5kuB"
	addrMain := "tz1LNGzjz8H9juHNrHLKbZ1fm7un3KJpxsFY"

	require.Nil(VerifyAssetKey(xtz))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(xtz)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(xtz))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToLower(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(xtz))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("5649ca42-eb5f-4c0e-ae28-d9a4e77eded3")), GenerateAssetId(xtz))
	require.Equal(crypto.NewHash([]byte("5649ca42-eb5f-4c0e-ae28-d9a4e77eded3")), TezosChainId)
	require.Equal(crypto.NewHash([]byte(TezosChainBase)), TezosChainId)
}
