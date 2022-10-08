package nervos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestValidation(t *testing.T) {
	assert := assert.New(t)

	ckb := "d243386e-6d84-42e6-be03-175be17bf275"
	tx := "0x92d028bf29a20769347b0e1ac5c27cbf087b22f97a85c695da758df204442f2b"
	addrMain := "ckb1qyqt8csrd4yg4el5etgkvt8rmdg923t8yagswneqnr"
	addrMain1 := "ckb1qypgyg7qdhpkv7wuuutaw0ujx9ty837rtewsu2q6lk"
	addrMain2 := "ckb1qzda0cr08m85hc8jlnfp3zer7xulejywt49kt2rr0vthywaa50xwsqd8j5pnrmedx62duaqtzwyuglprklwuxts88lh7h"
	addrMain3 := "ckb1qjfhdsa4syv599s2s3nfrctwga70g0tu07n9gpnun9ydlngf5vsnwq6le2u89fdsq4kdseycs4p9mycf699w0dcrtl9tsu49kqzkekrynzz5yhvnp8g54eahkzvfk2"

	assert.Nil(VerifyAssetKey(ckb))
	assert.NotNil(VerifyAssetKey(tx))
	assert.NotNil(VerifyAssetKey(addrMain))
	assert.NotNil(VerifyAssetKey(strings.ToUpper(ckb)))

	assert.Nil(VerifyAddress(addrMain))
	assert.Nil(VerifyAddress(addrMain1))
	assert.Nil(VerifyAddress(addrMain2))
	assert.Nil(VerifyAddress(addrMain3))
	assert.NotNil(VerifyAddress(ckb))
	assert.NotNil(VerifyAddress(addrMain[1:]))
	assert.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	assert.Nil(VerifyTransactionHash(tx))
	assert.NotNil(VerifyTransactionHash(ckb))
	assert.NotNil(VerifyTransactionHash(addrMain))
	assert.NotNil(VerifyTransactionHash("0x" + tx))
	assert.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	assert.Equal(crypto.NewHash([]byte("d243386e-6d84-42e6-be03-175be17bf275")), GenerateAssetId(ckb))
	assert.Equal(crypto.NewHash([]byte("d243386e-6d84-42e6-be03-175be17bf275")), NervosChainId)
	assert.Equal(crypto.NewHash([]byte(NervosChainBase)), NervosChainId)
}
