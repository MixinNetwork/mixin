package bitcoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	btc := "c6d0c728-2624-429b-8e0d-d9d19b6592fa"
	usdt := "815b0b1a-2764-3736-8faa-42d694fa620a"
	tx := "c5945a8571fc84cd6850b26b5771d76311ed56957a04e993927de07b83f07c91"
	addrLeg := "1zgmvYi5x1wy3hUh7AjKgpcVgpA8Lj9FA"
	addrSeg := "bc1qxenlll5m5zyp778j8jd6arkn99h956zkcye93n"
	addrCash := "qptz5xa5dd670f453grrplt6d4llaxlm05qmwktdc5"

	assert.Nil(VerifyAssetKey(btc))
	assert.Nil(VerifyAssetKey(usdt))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrLeg))
	assert.NotNil(VerifyAssetKey(addrSeg))
	assert.NotNil(VerifyAssetKey(addrCash))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(btc)))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(usdt)))

	assert.Nil(VerifyAddress(addrLeg))
	assert.Nil(VerifyAddress(addrSeg))
	assert.NotNil(VerifyAddress(btc))
	assert.NotNil(VerifyAddress(usdt))
	assert.NotNil(VerifyAddress(addrCash))
	assert.NotNil(VerifyAddress(addrLeg[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrLeg)))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrCash)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(btc))
	assert.NotNil(VerifyTransactionHash(addrLeg))
	assert.NotNil(VerifyTransactionHash(addrSeg))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("c6d0c728-2624-429b-8e0d-d9d19b6592fa")), GenerateAssetId(btc))
	assert.Equal(crypto.NewHash([]byte("815b0b1a-2764-3736-8faa-42d694fa620a")), GenerateAssetId(usdt))
	assert.Equal(crypto.NewHash([]byte("c6d0c728-2624-429b-8e0d-d9d19b6592fa")), BitcoinChainId)
	assert.Equal(crypto.NewHash([]byte("815b0b1a-2764-3736-8faa-42d694fa620a")), BitcoinOmniUSDTId)
}
