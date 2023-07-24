package common

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestCustodian(t *testing.T) {
	require := require.New(t)

	msg := "AXjpmilx5N4AV7TfZYPGJ880VjHctV76u1mGhVF9l9obUUv5TNnZbEC4beqfAS2e0RAGMQeM3o6i5KdZgl0sh3h7zwSeoiwoeh45wPxN0t96wibGBO3aTkBKpwRaOM6QpXoom3wIdG8o1Bquqv05SrNaOZSxD6EFlFR99loc9lTr_xnpMHU4RsZ2w0AELVVHAhtdWb4xgfRxt_18My1hNnJrIxUfmf4SYq_01tB8RE-GTC1pk7jqwQ6y5KjI3neGqL9xGCDa8FJPQOLkmCNSCWqdGRVEHGUD-Irj4oAt2OgOD4C2hPhgghT-Q7QBHEbXbhg7WFavLCO7PWK9eiE7c79DaZUw51-08tF2nh9RC5sK4AeqkbaiZ47efzbHrQ1kCxgH0Ra85_kSGwPW_sVvTeMRYKaE3oxT4UKeZAeqpb5XfsY2Zl-X9zqvYkAfZuSsRilcKu3pDgOolHWNcB3NjgM"
	extra, _ := base64.RawURLEncoding.DecodeString(msg)
	cn, err := ParseCustodianNode(extra)
	require.Nil(err)
	require.NotNil(cn)
	require.Equal("XINGpVSTGyPEmtXQUCaSEGbnq2ZBVgZxtej6gaVhZ5qm39kbPncsa6TPSjQ8WrPQSZt4Bd5ZvbbYrLZvqJWdZ1T7a1JCA7WK", cn.Custodian.String())
	require.Equal("XINHCU4KJj3XJT3shyYSoRp3RPQag3MaQc36xaDwqraVs6HZDu4r5t7vSHk6zm6rFmXENGMQcphq5ZhikwA5bfeZexXKqsof", cn.Payee.String())
	require.Equal(extra, cn.Extra)
	require.Nil(cn.validate())

	mainnet, _ := crypto.HashFromString(config.MainnetId)
	payee := testBuildAddress(require)
	signer := testBuildAddress(require)
	custodian := testBuildAddress(require)
	nodeId := signer.Hash().ForNetwork(mainnet)
	extra = EncodeCustodianNode(&custodian, &payee, &signer.PrivateSpendKey, &payee.PrivateSpendKey, &custodian.PrivateSpendKey, mainnet)
	cn, err = ParseCustodianNode(extra)
	require.Nil(err)
	require.NotNil(cn)
	require.Equal(custodian.String(), cn.Custodian.String())
	require.Equal(payee.String(), cn.Payee.String())
	require.Equal(extra, cn.Extra)
	require.Contains(hex.EncodeToString(extra), hex.EncodeToString(nodeId[:]))
	require.Nil(cn.validate())
}

func testBuildAddress(require *require.Assertions) Address {
	seed := make([]byte, 64)
	n, err := rand.Read(seed)
	require.Nil(err)
	require.Equal(64, n)
	addr := NewAddressFromSeed(seed)
	addr.PrivateViewKey = addr.PublicSpendKey.DeterministicHashDerive()
	addr.PublicViewKey = addr.PrivateViewKey.Public()
	return addr
}
