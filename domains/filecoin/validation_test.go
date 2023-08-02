package filecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	fil := "08285081-e1d8-4be6-9edc-e203afa932da"
	tx := "bafy2bzaceaqr65fthy3z4wn2rmo7ani75sekd5kwsg3pkrzznynopbgnovtkc"
	addrMain := "f1egh23o5qy2ibkqwawqyjague4urpxiyf672l6zi"

	require.Nil(VerifyAssetKey(fil))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(fil)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(fil))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(fil))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("08285081-e1d8-4be6-9edc-e203afa932da")), GenerateAssetId(fil))
	require.Equal(crypto.NewHash([]byte("08285081-e1d8-4be6-9edc-e203afa932da")), FilecoinChainId)
	require.Equal(crypto.NewHash([]byte(FilecoinChainBase)), FilecoinChainId)
}
