package dfinity

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	icp := "d5db6f39-fe50-4633-8edc-36e2f3e117e4"
	tx := "8614fec5bc43d40fbc252ac3b042b7a01d622338e073d790d2da501cab845a8c"
	addrMain := "449ce7ad1298e2ed2781ed379aba25efc2748d14c60ede190ad7621724b9e8b2"

	require.Nil(VerifyAssetKey(icp))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(icp)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(icp))
	require.NotNil(VerifyAddress(addrMain[1:]))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(icp))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("d5db6f39-fe50-4633-8edc-36e2f3e117e4")), GenerateAssetId(icp))
	require.Equal(crypto.NewHash([]byte("d5db6f39-fe50-4633-8edc-36e2f3e117e4")), DfinityChainId)
	require.Equal(crypto.NewHash([]byte(DfinityChainBase)), DfinityChainId)
}
