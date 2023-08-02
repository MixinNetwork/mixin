package zcash

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	zec := "c996abc9-d94e-4494-b1cf-2a3fd3ac5714"
	tx := "30f305889eab065bb5c85e724df9ffb1c8da7f22259c583cf874fbd6ec681b8a"
	addrMain := "t1NsuW4Xpz3GQUzt3BTZAxN6k4svKfWXgni"

	require.Nil(VerifyAssetKey(zec))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(zec)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(zec))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(zec))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("c996abc9-d94e-4494-b1cf-2a3fd3ac5714")), GenerateAssetId(zec))
	require.Equal(crypto.NewHash([]byte("c996abc9-d94e-4494-b1cf-2a3fd3ac5714")), ZcashChainId)
	require.Equal(crypto.NewHash([]byte(ZcashChainBase)), ZcashChainId)
}
