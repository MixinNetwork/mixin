package eos

import (
	"strings"
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestValidation(t *testing.T) {
	require := require.New(t)

	eos := "eosio.token:EOS"
	usdt := "tethertether:USDT"
	papaya := "eos.v2.wei:PAPAYA"
	tx := "197be13b8d572ae4c83fe2bc60e87ac8993896242bb486790fd4378f88d8d961"

	require.Nil(VerifyAssetKey(eos))
	require.Nil(VerifyAssetKey(usdt))
	require.Nil(VerifyAssetKey(papaya))
	require.NotNil(VerifyAssetKey("eosio.token"))
	require.NotNil(VerifyAssetKey("eosio.token.2"))
	require.NotNil(VerifyAssetKey("eos.io.token"))
	require.NotNil(VerifyAssetKey("eosio:EOSABCDEFG"))
	require.NotNil(VerifyAssetKey("eosio.token:EOS:2"))
	require.NotNil(VerifyAssetKey(strings.ToUpper(eos)))

	require.Nil(VerifyAddress("eosio.token"))
	require.Nil(VerifyAddress("tethertether"))
	require.Nil(VerifyAddress("eos.io.token"))
	require.Nil(VerifyAddress("eos.io.token.t"))
	require.NotNil(VerifyAddress("eos.io..token"))
	require.NotNil(VerifyAddress("eosio.token6"))
	require.NotNil(VerifyAddress("Eosio.token"))
	require.NotNil(VerifyAddress("eos.io.token.long"))
	require.NotNil(VerifyAddress("."))
	require.NotNil(VerifyAddress(".token"))
	require.NotNil(VerifyAddress("eosio."))
	require.NotNil(VerifyAddress("eosio.token."))
	require.NotNil(VerifyAddress(".eosio.token."))

	require.Nil(VerifyTransactionHash(tx))
	require.NotNil(VerifyTransactionHash(eos))
	require.NotNil(VerifyTransactionHash(tx[2:]))
	require.NotNil(VerifyTransactionHash(strings.ToUpper(tx)))

	require.Equal(crypto.NewHash([]byte("6cfe566e-4aad-470b-8c9a-2fd35b49c68d")), GenerateAssetId(eos))
	require.Equal(crypto.NewHash([]byte("5dac5e28-ad13-31ea-869f-41770dfcee09")), GenerateAssetId(usdt))
	require.Equal(crypto.NewHash([]byte("6cfe566e-4aad-470b-8c9a-2fd35b49c68d")), EOSChainId)
}
