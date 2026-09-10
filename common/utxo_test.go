package common

import (
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestUTXO(t *testing.T) {
	require := require.New(t)

	genesisHash := crypto.Hash{}
	script := Script{OperatorCmp, OperatorSum, 2}
	accounts := make([]*Address, 0)
	for range 3 {
		a := randomAccount()
		accounts = append(accounts, &a)
	}

	tx := NewTransactionV5(XINAssetId).AsVersioned()
	tx.AddInput(genesisHash, 0)
	tx.AddInput(genesisHash, 1)
	tx.AddRandomScriptOutput(accounts, script, NewInteger(20000))

	utxos := tx.UnspentOutputs()
	require.Len(utxos, 1)
	utxo := utxos[0]
	require.Equal(tx.PayloadHash(), utxo.Hash)
	require.Equal(uint(0), utxo.Index)
	require.Equal(uint8(OutputTypeScript), utxo.Type)
	require.Equal("20000.00000000", utxo.Amount.String())
	require.Equal("fffe02", utxo.Script.String())
	require.Len(utxo.Keys, 3)
	require.Equal(XINAssetId, utxo.Asset)
}
