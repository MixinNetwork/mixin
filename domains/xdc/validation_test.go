package xdc

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	xin := "xdc8f9920283470f52128bf11b0c14e798be704fd15"
	tx := "0x84326a8499614f7236ff22fd6d6aa473649c1704404ca86f37953394f4659127"

	xinFormat, _ := formatAddress(xin)
	require.Equal("xdc8f9920283470F52128bF11B0c14E798bE704fD15", xinFormat)

	require.Nil(VerifyAssetKey("xdc0000000000000000000000000000000000000000"))
	require.Nil(VerifyAssetKey(xin))
	require.NotNil(VerifyAssetKey(xinFormat))
	require.NotNil(VerifyAssetKey(xin[3:]))
	require.NotNil(VerifyAssetKey(strings.ToUpper(xin)))

	require.Nil(VerifyAddress(xinFormat))
	require.NotNil(VerifyAddress(xin))
	require.NotNil(VerifyAddress(xin[3:]))
	require.NotNil(VerifyAddress(strings.ToUpper(xin)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(xin))
	require.NotNil(VerifyTransactionHash(tx[2:]))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("b12bb04a-1cea-401c-a086-0be61f544889")), GenerateAssetId("xdc0000000000000000000000000000000000000000"))
	require.Equal(crypto.NewHash([]byte("b12bb04a-1cea-401c-a086-0be61f544889")), XDCChainId)
}
