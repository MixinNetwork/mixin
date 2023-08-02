package dogecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	doge := "6770a1e5-6086-44d5-b60f-545f9d9e8ffd"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "DANHz6EQVoWyZ9rER56DwTXHWUxfkv9k2o"

	require.Nil(VerifyAssetKey(doge))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(doge)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(doge))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(doge))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("6770a1e5-6086-44d5-b60f-545f9d9e8ffd")), GenerateAssetId(doge))
	require.Equal(crypto.NewHash([]byte("6770a1e5-6086-44d5-b60f-545f9d9e8ffd")), DogecoinChainId)
	require.Equal(crypto.NewHash([]byte(DogecoinChainBase)), DogecoinChainId)
}
