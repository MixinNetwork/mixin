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
	for i := 0; i < 3; i++ {
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
	require.Equal(tx.PayloadHash(), utxo.Input.Hash)
	require.Equal(0, utxo.Input.Index)
	require.Equal(uint8(OutputTypeScript), utxo.Output.Type)
	require.Equal("20000.00000000", utxo.Output.Amount.String())
	require.Equal("fffe02", utxo.Output.Script.String())
	require.Len(utxo.Output.Keys, 3)
	require.Equal(XINAssetId, utxo.Asset)
}
