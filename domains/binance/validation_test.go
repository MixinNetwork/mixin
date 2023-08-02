package binance

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	bnb := "BNB"
	tx := "752b23fa8585f2516022a481c6c57f42f355cbb79560e7f26520ddb027ecc48f"
	addrMain := "bnb1rmc2xnpgx48hfq5jr8hqzh02ewl26dz5k0vfu7"

	require.Nil(VerifyAssetKey(bnb))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToLower(bnb)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(bnb))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(bnb))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("17f78d7c-ed96-40ff-980c-5dc62fecbc85")), GenerateAssetId(bnb))
	require.Equal(crypto.NewHash([]byte("17f78d7c-ed96-40ff-980c-5dc62fecbc85")), BinanceChainId)
	require.Equal(crypto.NewHash([]byte(BinanceChainBase)), BinanceChainId)
	require.Equal(crypto.NewHash([]byte("f312d6a7-1b4d-34c0-bf84-75e657a3fcf3")), GenerateAssetId("BUSD-BD1"))
}
