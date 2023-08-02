package kusama

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	ksm := "9d29e4f6-d67c-4c4b-9525-604b04afbe9f"
	tx := "0x961c4418df4afdbc2dcca2a146e01eadc8a56f76515c523ee1bda55d46e4b3e0"
	addrMain := "F4xQKRUagnSGjFqafyhajLs94e7Vvzvr8ebwYJceKpr8R7T"

	require.Nil(VerifyAssetKey(ksm))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(ksm)))

	require.Nil(VerifyAddress(addrMain))
	require.NotNil(VerifyAddress(ksm))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(ksm))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("9d29e4f6-d67c-4c4b-9525-604b04afbe9f")), GenerateAssetId(ksm))
	require.Equal(crypto.NewHash([]byte("9d29e4f6-d67c-4c4b-9525-604b04afbe9f")), KusamaChainId)
	require.Equal(crypto.NewHash([]byte(KusamaChainBase)), KusamaChainId)
}
