package common

import (
	"testing"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/assert"
)

func TestUTXO(t *testing.T) {
	assert := assert.New(t)

	genesisHash := crypto.Hash{}
	script := Script{OperatorCmp, OperatorSum, 2}
	accounts := make([]*Address, 0)
	for i := 0; i < 3; i++ {
		a := randomAccount()
		accounts = append(accounts, &a)
	}

	tx := NewTransactionV3(XINAssetId).AsVersioned()
	tx.AddInput(genesisHash, 0)
	tx.AddInput(genesisHash, 1)
	tx.AddRandomScriptOutput(accounts, script, NewInteger(20000))

	utxos := tx.UnspentOutputs()
	assert.Len(utxos, 1)
	utxo := utxos[0]
	assert.Equal(tx.PayloadHash(), utxo.Input.Hash)
	assert.Equal(0, utxo.Input.Index)
	assert.Equal(uint8(OutputTypeScript), utxo.Output.Type)
	assert.Equal("20000.00000000", utxo.Output.Amount.String())
	assert.Equal("fffe02", utxo.Output.Script.String())
	assert.Len(utxo.Output.Keys, 3)
	assert.Equal(XINAssetId, utxo.Asset)

	res, err := DecompressUnmarshalUTXO(utxo.CompressMarshal())
	assert.Nil(err)
	assert.Equal(msgpackMarshalPanic(res), msgpackMarshalPanic(utxo))
}

func TestUTXOLegacy(t *testing.T) {
	assert := assert.New(t)

	genesisHash := crypto.Hash{}
	script := Script{OperatorCmp, OperatorSum, 2}
	accounts := make([]*Address, 0)
	for i := 0; i < 3; i++ {
		a := randomAccount()
		accounts = append(accounts, &a)
	}

	tx := NewTransactionV2(XINAssetId).AsVersioned()
	tx.AddInput(genesisHash, 0)
	tx.AddInput(genesisHash, 1)
	tx.AddRandomScriptOutput(accounts, script, NewInteger(20000))

	utxos := tx.UnspentOutputs()
	assert.Len(utxos, 1)
	utxo := utxos[0]
	assert.Equal(tx.PayloadHash(), utxo.Input.Hash)
	assert.Equal(0, utxo.Input.Index)
	assert.Equal(uint8(OutputTypeScript), utxo.Output.Type)
	assert.Equal("20000.00000000", utxo.Output.Amount.String())
	assert.Equal("fffe02", utxo.Output.Script.String())
	assert.Len(utxo.Output.Keys, 3)
	assert.Equal(XINAssetId, utxo.Asset)
}
