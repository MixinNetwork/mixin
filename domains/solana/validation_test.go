package solana

import (
	"crypto/md5"
	"io"
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	sol := "11111111111111111111111111111111"
	tx := "rhz84aQJvQaYquFuDuyHVUHq8kZBjHrsmFDHRM2r87rjygCNBk6F9GtCfiLL31juDM4YptXHMyVXbcnupELcu1N"
	addrMain := "GuscxHWgjxoMTokbW5bmt54WnHAVEtyE3RCVXgxdZjnG"
	splUSDC := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	require.Nil(VerifyAssetKey(sol))
	require.Nil(VerifyAssetKey(splUSDC))
	require.Nil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(tx))

	require.Nil(VerifyAddress(addrMain))
	require.Nil(VerifyAddress(sol))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(sol))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("64692c23-8971-4cf4-84a7-4dd1271dd887")), GenerateAssetId(sol))
	require.Equal(crypto.NewHash([]byte("64692c23-8971-4cf4-84a7-4dd1271dd887")), SolanaChainId)
	require.Equal(crypto.NewHash([]byte(SolanaChainBase)), SolanaChainId)
	assetId := GenerateAssetId(splUSDC)
	uid := uniqueAssetId(SolanaChainBase, splUSDC)
	result := crypto.NewHash([]byte(uid))
	require.Equal(assetId.String(), result.String())
}

func uniqueAssetId(chainId, assetAddress string) string {
	h := md5.New()
	io.WriteString(h, chainId)
	io.WriteString(h, assetAddress)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	return uuid.FromBytesOrNil(sum).String()
}
