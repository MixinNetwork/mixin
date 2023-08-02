package ripple

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	xrp := "23dfb5a5-5d7b-48b6-905f-3970e3176e27"
	tx := "564D15A614B47A01D9F3AD08EC298ED8D7A7ECC98F4D64627D4D6A559668DBC8"
	addrMain := "rK6Vezau2D1FDUhFs1me35H3xod8UKc1Go"

	require.Nil(VerifyAssetKey(xrp))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(xrp)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(xrp))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(xrp))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToLower(tx)))

	require.Equal(crypto.NewHash([]byte("23dfb5a5-5d7b-48b6-905f-3970e3176e27")), GenerateAssetId(xrp))
	require.Equal(crypto.NewHash([]byte("23dfb5a5-5d7b-48b6-905f-3970e3176e27")), RippleChainId)
	require.Equal(crypto.NewHash([]byte(RippleChainBase)), RippleChainId)
}
