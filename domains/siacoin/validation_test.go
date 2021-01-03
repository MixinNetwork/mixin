package siacoin

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	sc := "990c4c29-57e9-48f6-9819-7d986ea44985"
	tx := "a78040a7b25278a96dfcbf56f9e0945072188a3638db549481f52db8dfcaa647"
	addrMain := "7a029a98f4be2d5f0364b0c5bc27fa1a0c45a9ca670fab2109e6b8328969e0899b774cf91478"

	assert.Nil(VerifyAssetKey(sc))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(sc)))

	assert.Nil(VerifyAddress(addrMain))
	assert.NotNil(VerifyAddress(sc))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(sc))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("990c4c29-57e9-48f6-9819-7d986ea44985")), GenerateAssetId(sc))
	assert.Equal(crypto.NewHash([]byte("990c4c29-57e9-48f6-9819-7d986ea44985")), SiacoinChainId)
	assert.Equal(crypto.NewHash([]byte(SiacoinChainBase)), SiacoinChainId)
}
