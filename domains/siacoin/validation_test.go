package siacoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	sc := "990c4c29-57e9-48f6-9819-7d986ea44985"
	tx := "a78040a7b25278a96dfcbf56f9e0945072188a3638db549481f52db8dfcaa647"
	addrMain := "7a029a98f4be2d5f0364b0c5bc27fa1a0c45a9ca670fab2109e6b8328969e0899b774cf91478"

	require.Nil(VerifyAssetKey(sc))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(sc)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(sc))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(sc))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("990c4c29-57e9-48f6-9819-7d986ea44985")), GenerateAssetId(sc))
	require.Equal(crypto.NewHash([]byte("990c4c29-57e9-48f6-9819-7d986ea44985")), SiacoinChainId)
	require.Equal(crypto.NewHash([]byte(SiacoinChainBase)), SiacoinChainId)
}
