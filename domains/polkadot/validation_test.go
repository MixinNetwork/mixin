package polkadot

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	dot := "54c61a72-b982-4034-a556-0d99e3c21e39"
	tx := "0x69cb313180b82f8d98314fc57c09905acc82282df3d068091e2344ea35a85c5a"
	addrMain := "13eM4Bgw55j93P7tiozfSjCkr55imbbiyso9MTG6YiQLaZSt"

	require.Nil(VerifyAssetKey(dot))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(dot)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(dot))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(dot))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("54c61a72-b982-4034-a556-0d99e3c21e39")), GenerateAssetId(dot))
	require.Equal(crypto.NewHash([]byte("54c61a72-b982-4034-a556-0d99e3c21e39")), PolkadotChainId)
	require.Equal(crypto.NewHash([]byte(PolkadotChainBase)), PolkadotChainId)
}
