package monero

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	xmr := "05c5ac01-31f9-4a69-aa8a-ab796de1d041"
	tx := "b140a0c02836f56a3a0638d1bb9118b660701879b7307f26373e51756a3fb1f5"
	addrMain := "447XRzap95djHJ1eQPXH6a1atfkZ1LLeVbr36BEH5HJCZgESVsCwpZfLX413y7gECRPaKS3Wz3izkQcQzzfRre6ER4oKK1P"
	addrSub := "883UmfvPF1NezhWZuVwZBbP2WyE6Z6BceCekLae8uw3RfzZMUk6mpBkEcKKfQbSEUBhLq4dEhWsjJcnMTqSM9AMALtnVjsm"

	assert.Nil(VerifyAssetKey(xmr))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(addrSub))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(xmr)))

	assert.Nil(VerifyAddress(addrMain))
	assert.Nil(VerifyAddress(addrSub))
	assert.NotNil(VerifyAddress(xmr))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrSub)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(xmr))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash(addrSub))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("05c5ac01-31f9-4a69-aa8a-ab796de1d041")), GenerateAssetId(xmr))
	assert.Equal(crypto.NewHash([]byte("05c5ac01-31f9-4a69-aa8a-ab796de1d041")), MoneroChainId)
	assert.Equal(crypto.NewHash([]byte(MoneroChainBase)), MoneroChainId)
}
