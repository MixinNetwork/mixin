package aptos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	apt := "0x1::aptos_coin::AptosCoin"
	tx := "0xef850268f585c82a02deaedcfc3cc9962668a02209974b77adb1a7c5f0b974d5"
	addrMain := "0x89fa1b72e65fab3da9a42dfe28047c658a5f1ab8857daaf9621b62156baec9f4"

	require.Nil(VerifyAssetKey(apt))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(apt)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(apt))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(apt))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("d2c1c7e1-a1a9-4f88-b282-d93b0a08b42b")), GenerateAssetId(AptosAssetKey))
	require.Equal(crypto.NewHash([]byte("d2c1c7e1-a1a9-4f88-b282-d93b0a08b42b")), AptosChainId)
	require.Equal(crypto.NewHash([]byte(AptosChainBase)), AptosChainId)
}
