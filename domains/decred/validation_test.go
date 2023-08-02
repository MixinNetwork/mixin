package decred

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	dcr := "8f5caf2a-283d-4c85-832a-91e83bbf290b"
	tx := "00a1630c8d0af5ef875d1f13330cc64cee0f91bc5f5aee8e401bf13d2a1beb04"
	addrMain := "DsoBw7Xa2dh1pRYcmFC3npi4Mh4ZydbMzUH"

	require.Nil(VerifyAssetKey(dcr))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(dcr)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(dcr))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(dcr))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("8f5caf2a-283d-4c85-832a-91e83bbf290b")), GenerateAssetId(dcr))
	require.Equal(crypto.NewHash([]byte("8f5caf2a-283d-4c85-832a-91e83bbf290b")), DecredChainId)
	require.Equal(crypto.NewHash([]byte(DecredChainBase)), DecredChainId)
}
