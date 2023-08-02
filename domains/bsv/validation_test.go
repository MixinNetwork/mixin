package bsv

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	bsv := "574388fd-b93f-4034-a682-01c2bc095d17"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "19q6XbBBYLhxnQGxWeS3fiehV5huV8bAZd"

	require.Nil(VerifyAssetKey(bsv))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(bsv)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(bsv))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(bsv))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("574388fd-b93f-4034-a682-01c2bc095d17")), GenerateAssetId(bsv))
	require.Equal(crypto.NewHash([]byte("574388fd-b93f-4034-a682-01c2bc095d17")), BitcoinSVChainId)
	require.Equal(crypto.NewHash([]byte(BitcoinSVChainBase)), BitcoinSVChainId)
}
