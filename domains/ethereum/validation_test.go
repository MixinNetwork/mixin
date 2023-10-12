package ethereum

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	xin := "0xa974c709cfb4566686553a20790685a47aceaa33"
	usdt := "0xdac17f958d2ee523a2206206994597c13d831ec7"
	tx := "0xc5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"

	xinFormat, _ := formatAddress(xin)
	usdtFormat, _ := formatAddress(usdt)
	require.Equal("0xA974c709cFb4566686553a20790685A47acEAA33", xinFormat)
	require.Equal("0xdAC17F958D2ee523a2206206994597C13D831ec7", usdtFormat)

	require.Nil(VerifyAssetKey(xin))
	require.Nil(VerifyAssetKey(usdt))
	require.NotNil(VerifyAssetKey(xinFormat))
	require.NotNil(VerifyAssetKey(usdtFormat))
	require.NotNil(VerifyAssetKey(xin[2:]))
	require.NotNil(VerifyAssetKey(usdt[2:]))
	require.NotNil(VerifyAssetKey(strings.ToUpper(xin)))
	require.NotNil(VerifyAssetKey(strings.ToUpper(usdt)))

	require.Nil(VerifyAddress(xinFormat))
	require.Nil(VerifyAddress(usdtFormat))
	require.NotNil(VerifyAddress(xin))
	require.NotNil(VerifyAddress(usdt))
	require.NotNil(VerifyAddress(xin[2:]))
	require.NotNil(VerifyAddress(usdt[2:]))
	require.NotNil(VerifyAddress(strings.ToUpper(xin)))
	require.NotNil(VerifyAddress(strings.ToUpper(usdt)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(xin))
	require.NotNil(VerifyTransactionHash(tx[2:]))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.Sha256Hash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8")), GenerateAssetId(xin))
	require.Equal(crypto.Sha256Hash([]byte("4d8c508b-91c5-375b-92b0-ee702ed2dac5")), GenerateAssetId(usdt))
	require.Equal(crypto.Sha256Hash([]byte("43d61dcd-e413-450d-80b8-101d5e903357")), GenerateAssetId("0x0000000000000000000000000000000000000000"))
	require.Equal(crypto.Sha256Hash([]byte("43d61dcd-e413-450d-80b8-101d5e903357")), EthereumChainId)
}
