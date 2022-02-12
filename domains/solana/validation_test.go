package solana

import (
	"crypto/md5"
	"io"
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	sol := "11111111111111111111111111111111"
	tx := "rhz84aQJvQaYquFuDuyHVUHq8kZBjHrsmFDHRM2r87rjygCNBk6F9GtCfiLL31juDM4YptXHMyVXbcnupELcu1N"
	addrMain := "GuscxHWgjxoMTokbW5bmt54WnHAVEtyE3RCVXgxdZjnG"
	splUSDC := "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"

	assert.Nil(VerifyAssetKey(sol))
	assert.Nil(VerifyAssetKey(splUSDC))
	assert.Nil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(tx))

	assert.Nil(VerifyAddress(addrMain))
	assert.Nil(VerifyAddress(sol))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(sol))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("64692c23-8971-4cf4-84a7-4dd1271dd887")), GenerateAssetId(sol))
	assert.Equal(crypto.NewHash([]byte("64692c23-8971-4cf4-84a7-4dd1271dd887")), SolanaChainId)
	assert.Equal(crypto.NewHash([]byte(SolanaChainBase)), SolanaChainId)
	assetId := GenerateAssetId(splUSDC)
	uid := uniqueAssetId(SolanaChainBase, splUSDC)
	result := crypto.NewHash([]byte(uid))
	assert.Equal(assetId.String(), result.String())
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
