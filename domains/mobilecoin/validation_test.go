package mobilecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	mob := "eea900a8-b327-488c-8d8d-1428702fe240"
	tx := "40c7e63c8cd2ddb1e65ffd3531e47739ed78cdcfef9cfd5cb6916f3c50d19c16"
	addr := "G57w8Br44AYd6aEKfagTyLFvt4tTLhDdzGsX6PbYwfumwpjc1htSpWfoey2FLYNKMJA28q8YyqYb83dh66A7BTVA4XNZzXsNNUDv1nTmaw"
	addr2 := "d9V5WDNZxa7fNRw24JwaqDpCrRnKjpsGcN2CpLJNJZQjt7Vhwsmm2w2CoY4g2u7vy5HFxL5S8uGUUogApWteXdgrd3GnowaFUWU1HefMdGK7fdhWEcznR9SacnddL2KA3NmEzgDRoqqwHzTv8cPXo5udtRAy4Q4xYsPTmXkcZTH212SNXudQwA6KfwUqS3aKvJFMLcr1iUmfupMikwVYcfboJ6i3gejGhua5BVX1GRhL2BRWMHhnRCThqicQAy"

	require.Nil(VerifyAssetKey(mob))
	require.NotNil(VerifyAssetKey("MCIP0025:0"))
	require.Nil(VerifyAssetKey("MCIP0025:1"))
	require.NotNil(VerifyAssetKey("MCIP0025:2"))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addr))
	require.NotNil(VerifyAssetKey(strings.ToUpper(mob)))

	require.Nil(VerifyAddress(addr))
	require.Nil(VerifyAddress(addr2))
	require.NotNil(VerifyAddress("G57w8Br44AYd6aEKfagTy"))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(mob))
	require.NotNil(VerifyTransactionHash(addr))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("eea900a8-b327-488c-8d8d-1428702fe240")), GenerateAssetId(mob))
	require.Equal("099fb16c3f8523bcb77c1e6c2bdb96f114611993e70b610dc1f5cfb3f273cbb1", GenerateAssetId("MCIP0025:1").String())
	require.Equal(crypto.NewHash([]byte("eea900a8-b327-488c-8d8d-1428702fe240")), MobileCoinChainId)
	require.Equal(crypto.NewHash([]byte(MobileCoinChainBase)), MobileCoinChainId)
}
