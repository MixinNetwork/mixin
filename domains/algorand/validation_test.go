package algorand

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	algo := "706b6f84-3333-4e55-8e89-275e71ce9803"
	tx := "OLY6AWDB7QCUQZWMVTPUIVTI65SNXSVU7OKLGXLGZSIWOSJMIWFQ"
	addrMain := "KZRF5B5JGH2NGSEG3DSKYM4KBB2OCDZY3BGXYCAZTMJBADDISJ436DNDTM"

	assert.Nil(VerifyAssetKey(algo))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(algo)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(algo))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToLower(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(algo))
	assert.NotNil(VerifyTransactionHash("0x" + tx))

	assert.Equal(crypto.NewHash([]byte("706b6f84-3333-4e55-8e89-275e71ce9803")), GenerateAssetId(algo))
	assert.Equal(crypto.NewHash([]byte("706b6f84-3333-4e55-8e89-275e71ce9803")), AlgorandChainId)
	assert.Equal(crypto.NewHash([]byte(AlgorandChainBase)), AlgorandChainId)
}
