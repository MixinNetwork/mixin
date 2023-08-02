package horizen

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	zen := "a2c5d22b-62a2-4c13-b3f0-013290dbac60"
	tx := "8c30eece44c9b4f4314f06ec5eedc7486e83ae76159ea81a0ee7aac2f16bbf0b"
	addrMain := "zszpcLB6C5B8QvfDbF2dYWXsrpac5DL9WRk"

	require.Nil(VerifyAssetKey(zen))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(zen)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(zen))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(zen))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("a2c5d22b-62a2-4c13-b3f0-013290dbac60")), GenerateAssetId(zen))
	require.Equal(crypto.NewHash([]byte("a2c5d22b-62a2-4c13-b3f0-013290dbac60")), HorizenChainId)
	require.Equal(crypto.NewHash([]byte(HorizenChainBase)), HorizenChainId)
}
