package external

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBlockHeight(t *testing.T) {
	assert := assert.New(t)

	height, err := GetBlockHeight(EthereumChainId)
	assert.Nil(err)
	assert.True(height > 5300000)
}

func TestGetBlock(t *testing.T) {
	assert := assert.New(t)

	block, err := GetBlock(EthereumChainId, 5300000)
	assert.Nil(err)
	assert.NotNil(block)
	assert.Equal(int64(5300000), block.BlockNumber)
	assert.Equal("0x7923d134566f0e6327964df01567f7b59cc1bc6c0e2b5da30702bc7449bd6178", block.BlockHash)
	assert.Equal(23, len(block.Transactions))

	block, err = GetBlock(EthereumChainId, 5306491)
	assert.Nil(err)
	assert.NotNil(block)
	assert.Equal(int64(5306491), block.BlockNumber)
	assert.Equal("0xbdb33a6d588fba1ca6385d1ac94e004b804c255c1b64eacaf831c0e82930aa37", block.BlockHash)
	assert.Equal(220, len(block.Transactions))

	tx := block.Transactions[0]
	assert.Equal("0x3928cb46e68fe72ab9a3e968877536f1cf5c935bf58324acf5b3b04c4b0d4215", tx.TransactionHash)
	assert.Equal(int64(5306491), tx.BlockNumber)
	assert.Equal("0xbdb33a6d588fba1ca6385d1ac94e004b804c255c1b64eacaf831c0e82930aa37", tx.BlockHash)
	assert.Equal("4167.924785", tx.Amount.Persist())
	assert.Equal("0x95cCDD086daaC56cD886C15bfa58647314eB4082", tx.Sender)
	assert.Equal("0x2eEaC205F1270072873549555a9F121C268f322f", tx.Receiver)
	assert.Equal(EthereumChainId, tx.Asset.ChainId)
	assert.Equal("aa35c57e-f862-31d4-8881-5a9a98ae6de0", tx.Asset.AssetId)
	assert.Equal("0xecfe2dce2a5585614379fa67108cabb18a24a125", tx.Asset.ChainAssetKey)
	assert.Equal("CHI", tx.Asset.Symbol)
	assert.Equal("CHI", tx.Asset.Name)

	tx = block.Transactions[1]
	assert.Equal("0xe8afad38b7c138e792f86341a13b6c1cb531a45b52a8874391b7fde198c7720f", tx.TransactionHash)
	assert.Equal(int64(5306491), tx.BlockNumber)
	assert.Equal("0xbdb33a6d588fba1ca6385d1ac94e004b804c255c1b64eacaf831c0e82930aa37", tx.BlockHash)
	assert.Equal("20", tx.Amount.Persist())
	assert.Equal("0x5adc335aFA4f6F97d5920996f0f0fF06ACe3b998", tx.Sender)
	assert.Equal("0x7123659B5EA1Dd1fD6Bc579E6E825B636c8cb105", tx.Receiver)
	assert.Equal(EthereumChainId, tx.Asset.ChainId)
	assert.Equal("0bbf7e08-aa87-327d-8662-b4bba46a6fd3", tx.Asset.AssetId)
	assert.Equal("0xb4c55b5a1faf5323e59842171c2492773a3783dd", tx.Asset.ChainAssetKey)
	assert.Equal("BCDC", tx.Asset.Symbol)
	assert.Equal("BCDC Token", tx.Asset.Name)
}

func TestGetTransactionResult(t *testing.T) {
	assert := assert.New(t)
	tx, err := GetTransactionResult(EthereumChainId, "0x7c5ea5c837450f9a52fdc8322c515760781dd7c87a69fe1b964c5a2dda44cbbc")
	assert.Nil(err)
	assert.True(tx.Confirmations > 100)
	assert.Equal(TransactionReceiptFailed, tx.Receipt)
	assert.Equal("0.000735", tx.Fee.Persist())

	tx, err = GetTransactionResult(EthereumChainId, "0xf64f466dfeaab0a0d71ff9e9e347e298dbda4291c4e5117005afcec4655d64d0")
	assert.Nil(err)
	assert.True(tx.Confirmations > 100)
	assert.Equal(TransactionReceiptSuccessful, tx.Receipt)
	assert.Equal("0.00010671", tx.Fee.Persist())
}
