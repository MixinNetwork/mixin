package stellar

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	xlm := "56e63c06-b506-4ec5-885a-4a5ac17b83c1"
	tx := "fa01f7b2391eac01662316f1611be34611c28bd4746026f69b89ad86e9b9f581"
	addrMain := "GD77JOIFC622O5HXU446VIKGR5A5HMSTAUKO2FSN5CIVWPHXDBGIAG7Y"
	addrMain1 := "MD77JOIFC622O5HXU446VIKGR5A5HMSTAUKO2FSN5CIVWPHXDBGIAAAAAAAAAAAAADCO2"

	require.Nil(VerifyAssetKey(xlm))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(xlm)))

	require.Nil(VerifyAddress(addrMain))
	require.Nil(VerifyAddress(addrMain1))
	require.NotNil(VerifyAddress(xlm))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToLower(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(xlm))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("56e63c06-b506-4ec5-885a-4a5ac17b83c1")), GenerateAssetId(xlm))
	require.Equal(crypto.NewHash([]byte("56e63c06-b506-4ec5-885a-4a5ac17b83c1")), StellarChainId)
	require.Equal(crypto.NewHash([]byte(StellarChainBase)), StellarChainId)
}
