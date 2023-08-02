package handshake

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	hns := "13036886-6b83-4ced-8d44-9f69151587bf"
	tx := "8c30eece44c9b4f4314f06ec5eedc7486e83ae76159ea81a0ee7aac2f16bbf0b"
	addrMain := "hs1qsh9v47p3k75lk9js8dptdd4qcy3n0scd33lm4j"

	require.Nil(VerifyAssetKey(hns))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(hns)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(hns))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(hns))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("13036886-6b83-4ced-8d44-9f69151587bf")), GenerateAssetId(hns))
	require.Equal(crypto.NewHash([]byte("13036886-6b83-4ced-8d44-9f69151587bf")), HandshakenChainId)
	require.Equal(crypto.NewHash([]byte(HandshakenChainBase)), HandshakenChainId)
}
