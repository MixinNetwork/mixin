package bch

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	bch := "fd11b6e3-0b87-41f1-a41f-f0e9b49e5bf0"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "19q6XbBBYLhxnQGxWeS3fiehV5huV8bAZd"
	addrNew := "bitcoincash:pp8skudq3x5hzw8ew7vzsw8tn4k8wxsqsv0lt0mf3g"

	require.Nil(VerifyAssetKey(bch))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(bch)))

	require.Nil(VerifyAddress(addrMain))
	require.Nil(VerifyAddress(addrNew))
	require.NotNil(VerifyAddress(bch))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(bch))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("fd11b6e3-0b87-41f1-a41f-f0e9b49e5bf0")), GenerateAssetId(bch))
	require.Equal(crypto.NewHash([]byte("fd11b6e3-0b87-41f1-a41f-f0e9b49e5bf0")), BitcoinCashChainId)
	require.Equal(crypto.NewHash([]byte(BitcoinCashChainBase)), BitcoinCashChainId)
}
