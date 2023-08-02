package polygon

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	tusd := "0x2e1ad108ff1d8c782fcbbb89aad783ac49586756"
	usdc := "0x2791bca1f2de4661ed88a30c99a7a9449aa84174"
	tx := "0xc5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"

	tusdFormat, _ := formatAddress(tusd)
	usdcFormat, _ := formatAddress(usdc)
	require.Equal("0x2e1AD108fF1D8C782fcBbB89AAd783aC49586756", tusdFormat)
	require.Equal("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174", usdcFormat)

	require.Nil(VerifyAssetKey(tusd))
	require.Nil(VerifyAssetKey(usdc))
	require.NotNil(VerifyAssetKey(tusdFormat))
	require.NotNil(VerifyAssetKey(usdcFormat))
	require.NotNil(VerifyAssetKey(tusd[2:]))
	require.NotNil(VerifyAssetKey(usdc[2:]))
	require.NotNil(VerifyAssetKey(strings.ToUpper(tusd)))
	require.NotNil(VerifyAssetKey(strings.ToUpper(usdc)))

	require.Nil(VerifyAddress(tusdFormat))
	require.Nil(VerifyAddress(usdcFormat))
	require.NotNil(VerifyAddress(tusd))
	require.NotNil(VerifyAddress(usdc))
	require.NotNil(VerifyAddress(tusd[2:]))
	require.NotNil(VerifyAddress(usdc[2:]))
	require.NotNil(VerifyAddress(strings.ToUpper(tusd)))
	require.NotNil(VerifyAddress(strings.ToUpper(usdc)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(tusd))
	require.NotNil(VerifyTransactionHash(tx[2:]))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("9189a528-c3a5-36cb-8e08-feb81e7cb9cb")), GenerateAssetId(tusd))
	require.Equal(crypto.NewHash([]byte("80b65786-7c75-3523-bc03-fb25378eae41")), GenerateAssetId(usdc))
	require.Equal(crypto.NewHash([]byte("b7938396-3f94-4e0a-9179-d3440718156f")), GenerateAssetId("0x0000000000000000000000000000000000000000"))
	require.Equal(crypto.NewHash([]byte("b7938396-3f94-4e0a-9179-d3440718156f")), PolygonChainId)
}
