package bitcoin

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	btc := "c6d0c728-2624-429b-8e0d-d9d19b6592fa"
	tx := "c5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"
	addrLeg := "1zgmvYi5x1wy3hUh7AjKgpcVgpA8Lj9FA"
	addrSeg := "bc1qxenlll5m5zyp778j8jd6arkn99h956zkcye93n"
	addrTaproot := "bc1paardr2nczq0rx5rqpfwnvpzm497zvux64y0f7wjgcs7xuuuh2nnqwr2d5c"
	addrCash := "qptz5xa5dd670f453grrplt6d4llaxlm05qmwktdc5"

	require.Nil(VerifyAssetKey(btc))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrLeg))
	require.NotNil(VerifyAssetKey(addrSeg))
	require.NotNil(VerifyAssetKey(addrTaproot))
	require.NotNil(VerifyAssetKey(addrCash))
	require.NotNil(VerifyAssetKey(strings.ToUpper(btc)))

	require.Nil(VerifyAddress(addrLeg))
	require.Nil(VerifyAddress(addrSeg))
	require.Nil(VerifyAddress(addrTaproot))
	require.NotNil(VerifyAddress(btc))
	require.NotNil(VerifyAddress(addrCash))
	require.NotNil(VerifyAddress(addrLeg[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrLeg)))
	require.NotNil(VerifyAddress(strings.ToUpper(addrCash)))

	invalidAddr := "bc0100"
	require.NotNil(VerifyAddress(invalidAddr))
	ib, _ := hex.DecodeString(invalidAddr)
	require.NotNil(VerifyAddress(string(ib)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(btc))
	require.NotNil(VerifyTransactionHash(addrLeg))
	require.NotNil(VerifyTransactionHash(addrSeg))
	require.NotNil(VerifyTransactionHash(addrTaproot))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.Sha256Hash([]byte("c6d0c728-2624-429b-8e0d-d9d19b6592fa")), GenerateAssetId(btc))
	require.Equal(crypto.Sha256Hash([]byte("c6d0c728-2624-429b-8e0d-d9d19b6592fa")), BitcoinChainId)
}
