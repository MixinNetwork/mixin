package nervos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	ckb := "d243386e-6d84-42e6-be03-175be17bf275"
	tx := "0x92d028bf29a20769347b0e1ac5c27cbf087b22f97a85c695da758df204442f2b"
	addrMain := "ckb1qyqt8csrd4yg4el5etgkvt8rmdg923t8yagswneqnr"
	addrMain1 := "ckb1qypgyg7qdhpkv7wuuutaw0ujx9ty837rtewsu2q6lk"
	addrMain2 := "ckb1qzda0cr08m85hc8jlnfp3zer7xulejywt49kt2rr0vthywaa50xwsqd8j5pnrmedx62duaqtzwyuglprklwuxts88lh7h"
	addrMain3 := "ckb1qjfhdsa4syv599s2s3nfrctwga70g0tu07n9gpnun9ydlngf5vsnwq6le2u89fdsq4kdseycs4p9mycf699w0dcrtl9tsu49kqzkekrynzz5yhvnp8g54eahkzvfk2"

	require.Nil(VerifyAssetKey(ckb))
	require.NotNil(VerifyAssetKey(tx))
	require.NotNil(VerifyAssetKey(addrMain))
	require.NotNil(VerifyAssetKey(strings.ToUpper(ckb)))

	require.Nil(VerifyAddress(addrMain))
	require.Nil(VerifyAddress(addrMain1))
	require.Nil(VerifyAddress(addrMain2))
	require.Nil(VerifyAddress(addrMain3))
	require.NotNil(VerifyAddress(ckb))
	require.NotNil(VerifyAddress(addrMain[1:]))
	require.NotNil(VerifyAddress(strings.ToUpper(addrMain)))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(ckb))
	require.NotNil(VerifyTransactionHash(addrMain))
	require.NotNil(VerifyTransactionHash("0x" + tx))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("d243386e-6d84-42e6-be03-175be17bf275")), GenerateAssetId(ckb))
	require.Equal(crypto.NewHash([]byte("d243386e-6d84-42e6-be03-175be17bf275")), NervosChainId)
	require.Equal(crypto.NewHash([]byte(NervosChainBase)), NervosChainId)
}
