package tron

import (
	"crypto/md5"
	"io"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	err := VerifyAssetKey("25dabac5-056a-48ff-b9f9-f67395dc407c")
	require.Nil(err)
	err = VerifyAssetKey("43d61dcd-e413-450d-80b8-101d5e903357")
	require.NotNil(err)
	err = VerifyAssetKey("1002000")
	require.Nil(err)
	err = VerifyAssetKey("100200i")
	require.NotNil(err)
	err = VerifyAssetKey("10020001")
	require.NotNil(err)
	err = VerifyAssetKey("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
	require.Nil(err)
	err = VerifyAssetKey("Tr7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
	require.NotNil(err)

	err = VerifyAddress("TBJSVkP9zNDmHwnZtZHqG1GZXtWuJL71Mv")
	require.Nil(err)
	err = VerifyAddress("TBJSVkP9zNDmHwnZtZHqG1GZXtWuJL71M")
	require.NotNil(err)
	err = VerifyAddress("27QQk34hXSWEzz82oDQw7Kv8JKozWnbEGV3")
	require.NotNil(err)

	err = VerifyTransactionHash("f5eade17b339ae39e8d6b61cb1d935c942fae4e7da312e16fac2f1573d152dfe")
	require.Nil(err)
	err = VerifyTransactionHash("4fde7407d05d5895c296c6b5d3ab29bbec7494c1e464d17efacf8b8b1b210478")
	require.Nil(err)
	err = VerifyTransactionHash("4fde7407d05d5895c296c6b5d3ab29bbec7494c1e464d17efacf8b8b1b21047")
	require.NotNil(err)

	assetId := GenerateAssetId("25dabac5-056a-48ff-b9f9-f67395dc407c")
	require.Equal(assetId, TronChainId)
	assetId = GenerateAssetId("1002000")
	require.Equal(assetId.String(), "b052fbe0e3a8037d33556f2f80ef8847fd3d393181df5c7de47c4dccb7d55442")
	require.NotEqual(assetId.String(), "b152fbe0e3a8037d33556f2f80ef8847fd3d393181df5c7de47c4dccb7d55442")
	uid := uniqueAssetId(TronChainBase, "1002000")
	result := crypto.NewHash([]byte(uid))
	require.Equal(assetId.String(), result.String())
}

func uniqueAssetId(chainId, assetAddress string) string {
	h := md5.New()
	io.WriteString(h, chainId)
	io.WriteString(h, assetAddress)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	return uuid.FromBytesOrNil(sum).String()
}
