package arbitrum

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	xin := "0xa974c709cfb4566686553a20790685a47aceaa33"
	tx := "0xc5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"

	xinFormat, _ := formatAddress(xin)
	require.Equal("0xA974c709cFb4566686553a20790685A47acEAA33", xinFormat)

	require.Nil(VerifyAssetKey("0x0000000000000000000000000000000000000000"))
	require.Nil(VerifyAssetKey(xin))
	require.NotNil(VerifyAssetKey(xinFormat))
	require.NotNil(VerifyAssetKey(xin[2:]))
	require.NotNil(VerifyAssetKey(strings.ToUpper(xin)))

	require.Nil(VerifyAddress(xinFormat))
	require.NotNil(VerifyAddress(xin))
	require.NotNil(VerifyAddress(xin[2:]))
	require.NotNil(VerifyAddress(strings.ToUpper(xin)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(xin))
	require.NotNil(VerifyTransactionHash(tx[2:]))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("d0688ff7-6656-4a79-bd5f-d764bfa9bc46")), GenerateAssetId("0x0000000000000000000000000000000000000000"))
	require.Equal(crypto.NewHash([]byte("d0688ff7-6656-4a79-bd5f-d764bfa9bc46")), ArbitrumChainId)
}
