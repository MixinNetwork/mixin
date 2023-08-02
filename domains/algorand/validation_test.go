package algorand

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	algo := "706b6f84-3333-4e55-8e89-275e71ce9803"
	tx := "OLY6AWDB7QCUQZWMVTPUIVTI65SNXSVU7OKLGXLGZSIWOSJMIWFQ"
	addrMain := "KZRF5B5JGH2NGSEG3DSKYM4KBB2OCDZY3BGXYCAZTMJBADDISJ436DNDTM"

	require.Nil(VerifyAssetKey(algo))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(algo)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(algo))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToLower(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(algo))
	require.NotNil(VerifyTransactionHash("0x" + tx))

	require.Equal(crypto.NewHash([]byte("706b6f84-3333-4e55-8e89-275e71ce9803")), GenerateAssetId(algo))
	require.Equal(crypto.NewHash([]byte("706b6f84-3333-4e55-8e89-275e71ce9803")), AlgorandChainId)
	require.Equal(crypto.NewHash([]byte(AlgorandChainBase)), AlgorandChainId)
}
