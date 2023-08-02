package mvm

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	xin := "0x034a771797a1c8694bc33e1aa89f51d1f828e5a4"
	tx := "0xc5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"

	xinFormat, _ := formatAddress(xin)
	require.Equal("0x034A771797a1C8694Bc33E1AA89f51d1f828e5A4", xinFormat)

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

	require.Equal(crypto.NewHash([]byte("9437ebfa-873a-3032-8295-eedb8a3c86c7")), GenerateAssetId(xin))
	require.Equal(crypto.NewHash([]byte("a0ffd769-5850-4b48-9651-d2ae44a3e64d")), GenerateAssetId("0x0000000000000000000000000000000000000000"))
	require.Equal(crypto.NewHash([]byte("a0ffd769-5850-4b48-9651-d2ae44a3e64d")), MVMChainId)
}
