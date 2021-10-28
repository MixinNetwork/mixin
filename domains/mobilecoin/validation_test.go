package mobilecoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	mob := "eea900a8-b327-488c-8d8d-1428702fe240"
	tx := "40c7e63c8cd2ddb1e65ffd3531e47739ed78cdcfef9cfd5cb6916f3c50d19c16"
	addr := "G57w8Br44AYd6aEKfagTyLFvt4tTLhDdzGsX6PbYwfumwpjc1htSpWfoey2FLYNKMJA28q8YyqYb83dh66A7BTVA4XNZzXsNNUDv1nTmaw"
	addr2 := "d9V5WDNZxa7fNRw24JwaqDpCrRnKjpsGcN2CpLJNJZQjt7Vhwsmm2w2CoY4g2u7vy5HFxL5S8uGUUogApWteXdgrd3GnowaFUWU1HefMdGK7fdhWEcznR9SacnddL2KA3NmEzgDRoqqwHzTv8cPXo5udtRAy4Q4xYsPTmXkcZTH212SNXudQwA6KfwUqS3aKvJFMLcr1iUmfupMikwVYcfboJ6i3gejGhua5BVX1GRhL2BRWMHhnRCThqicQAy"

	assert.Nil(VerifyAssetKey(mob))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addr))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(mob)))

	assert.Nil(VerifyAddress(addr))
	assert.Nil(VerifyAddress(addr2))
	assert.NotNil(VerifyAddress("G57w8Br44AYd6aEKfagTy"))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(mob))
	assert.NotNil(VerifyTransactionHash(addr))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("eea900a8-b327-488c-8d8d-1428702fe240")), GenerateAssetId(mob))
	assert.Equal(crypto.NewHash([]byte("eea900a8-b327-488c-8d8d-1428702fe240")), MobileCoinChainId)
	assert.Equal(crypto.NewHash([]byte(MobileCoinChainBase)), MobileCoinChainId)
}
