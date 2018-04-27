package external

import "mixin.one/number"

type Asset struct {
	AssetId       string `json:"asset_id"`
	ChainId       string `json:"chain_id"`
	ChainAssetKey string `json:"chain_asset_key"`
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	Precision     int64  `json:"precision"`
}

type Transaction struct {
	Asset           Asset          `json:"asset"`
	TransactionHash string         `json:"transaction_hash"`
	Sender          string         `json:"sender"`
	Receiver        string         `json:"receiver"`
	Memo            string         `json:"memo"`
	BlockHash       string         `json:"block_hash"`
	BlockNumber     int64          `json:"block_number"`
	OutputIndex     int64          `json:"output_index"`
	Confirmations   int64          `json:"confirmations"`
	Amount          number.Decimal `json:"amount"`
	Fee             number.Decimal `json:"fee"`
	Receipt         string         `json:"receipt"`
}

type Block struct {
	BlockHash    string         `json:"block_hash"`
	BlockNumber  int64          `json:"block_number"`
	Transactions []*Transaction `json:"transactions"`
}
