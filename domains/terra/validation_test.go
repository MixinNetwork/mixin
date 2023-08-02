package terra

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	assetKey := "uluna"
	assetUsdKey := "uusd"
	anc := "terra14z56l0fp2lsf86zy3hty2z47ezkhnthtr9yq76"
	tx := "99a2a8bcd5da27cc910649f03259c8446a76d6345973be3922026c3dee9bcb1f"
	addrMain := "terra158n5uhvygpz5ttunfuaqh0l2ly5vhl72fy7d8q"

	require.Nil(VerifyAssetKey(assetKey))
	require.Nil(VerifyAssetKey(assetUsdKey))
	require.Nil(VerifyAssetKey(anc))
	require.Nil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(tx))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(assetKey))
	require.NotNil(VerifyAddress(assetUsdKey))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(assetKey))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("eb5bb26d-bfda-4e63-bf1d-a462b78343b7")), GenerateAssetId(assetKey))
	require.Equal(crypto.NewHash([]byte("cd54d4a2-6b64-3fe2-a1bc-16bb26deb2a3")), GenerateAssetId(anc))
	require.Equal(crypto.NewHash([]byte(TerraChainBase)), TerraChainId)
}
