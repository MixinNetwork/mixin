package monero

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	xmr := "05c5ac01-31f9-4a69-aa8a-ab796de1d041"
	tx := "b140a0c02836f56a3a0638d1bb9118b660701879b7307f26373e51756a3fb1f5"
	addrMain := "447XRzap95djHJ1eQPXH6a1atfkZ1LLeVbr36BEH5HJCZgESVsCwpZfLX413y7gECRPaKS3Wz3izkQcQzzfRre6ER4oKK1P"
	addrSub := "883UmfvPF1NezhWZuVwZBbP2WyE6Z6BceCekLae8uw3RfzZMUk6mpBkEcKKfQbSEUBhLq4dEhWsjJcnMTqSM9AMALtnVjsm"

	require.Nil(VerifyAssetKey(xmr))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(addrSub))
	require.NotNil(VerifyAssetKey(strings.ToUpper(xmr)))

	require.Nil(VerifyAddress(addrMain))
	require.Nil(VerifyAddress(addrSub))
	require.NotNil(VerifyAddress(xmr))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))
	require.NotNil(VerifyAddress(strings.ToUpper(addrSub)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(xmr))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash(addrSub))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("05c5ac01-31f9-4a69-aa8a-ab796de1d041")), GenerateAssetId(xmr))
	require.Equal(crypto.NewHash([]byte("05c5ac01-31f9-4a69-aa8a-ab796de1d041")), MoneroChainId)
	require.Equal(crypto.NewHash([]byte(MoneroChainBase)), MoneroChainId)
}
