package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"mixin.one/blockchain/external"
	"mixin.one/number"
)

// wget https://download.litecoin.org/litecoin-0.15.1/linux/litecoin-0.15.1-x86_64-linux-gnu.tar.gz
//
// server=1
// daemon=1
// txindex=1
// testnet=0

// rpcuser=2deca196257ec90d2aca14acffe25014
// rpcpassword=f83de9a2f5ef56221db2c529f525f15e2d2e2e9cd2a02d5adc2c9a97c7aff1a8
// rpcport=9332
// rpcallowip=10.0.0.0/8

const (
	litecoinMinimumHeight        = 100000
	litecoinHost                 = "litecoin-full-node:9332"
	litecoinUsername             = "2deca196257ec90d2aca14acffe25014"
	litecoinPassword             = "f83de9a2f5ef56221db2c529f525f15e2d2e2e9cd2a02d5adc2c9a97c7aff1a8"
	litecoinScriptPubKeyTypeHash = "pubkeyhash"
)

type RPC struct {
	client   *http.Client
	host     string
	username string
	password string
	id       string
}

func NewRPC() (*RPC, error) {
	chain := &RPC{
		client:   new(http.Client),
		host:     litecoinHost,
		username: litecoinUsername,
		password: litecoinPassword,
		id:       external.LitecoinChainId,
	}
	height, err := chain.GetBlockHeight()
	if err != nil {
		return nil, err
	}
	if height < litecoinMinimumHeight {
		return nil, fmt.Errorf("Litecoin block height too small %d", height)
	}
	return chain, nil
}

func (chain *RPC) GetBlockHeight() (int64, error) {
	body, err := chain.call("getblockchaininfo", []interface{}{})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Result struct {
			Blocks int64 `json:"blocks"`
		} `json:"result"`
		Error *LitecoinError `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return 0, err
	}
	if resp.Error != nil {
		return 0, resp.Error
	}
	if resp.Result.Blocks < litecoinMinimumHeight {
		return 0, fmt.Errorf("Litecoin block height too small %d", resp.Result.Blocks)
	}
	return resp.Result.Blocks, nil
}

func (chain *RPC) EstimateSmartFee() (number.Decimal, error) {
	body, err := chain.call("estimatesmartfee", []interface{}{2})
	if err != nil {
		return number.Zero(), err
	}
	var resp struct {
		Result struct {
			FeeRate float64 `json:"feerate"`
		} `json:"result"`
		Error *LitecoinError `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return number.Zero(), err
	}
	if resp.Error != nil {
		return number.Zero(), resp.Error
	}
	min := number.FromString("0.0001")
	fee := number.FromString(fmt.Sprint(resp.Result.FeeRate)).Mul(number.FromString("2"))
	if fee.Cmp(min) < 0 {
		fee = min
	}
	return fee.Round(8), nil
}

func (chain *RPC) GetBlock(ctx context.Context, id string) (*external.Block, error) {
	blockNumber, _ := strconv.ParseInt(id, 10, 64)
	if blockNumber > litecoinMinimumHeight {
		return chain.GetBlockByNumber(ctx, blockNumber)
	}
	return chain.GetBlockByHash(ctx, id)
}

func (chain *RPC) GetBlockByHash(ctx context.Context, blockHash string) (*external.Block, error) {
	body, err := chain.call("getblock", []interface{}{blockHash})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result LitecoinBlock  `json:"result"`
		Error  *LitecoinError `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	asset := external.Asset{
		ChainId:       chain.id,
		AssetId:       chain.id,
		ChainAssetKey: chain.id,
		Symbol:        "LTC",
		Name:          "Litecoin",
		Precision:     8,
	}
	block := &external.Block{
		BlockHash:    resp.Result.Hash,
		BlockNumber:  resp.Result.Height,
		Transactions: make([]*external.Transaction, 0),
	}
	for _, txId := range resp.Result.Tx {
		tx, err := chain.getRawTransaction(txId)
		if err != nil {
			return nil, err
		}
		for _, out := range tx.Vout {
			if out.ScriptPubKey.Type != litecoinScriptPubKeyTypeHash || len(out.ScriptPubKey.Addresses) != 1 {
				continue
			}
			amount := number.FromString(fmt.Sprint(out.Value))
			if amount.Exhausted() {
				continue
			}
			block.Transactions = append(block.Transactions, &external.Transaction{
				Asset:           asset,
				TransactionHash: tx.TxId,
				Sender:          "",
				Receiver:        out.ScriptPubKey.Addresses[0],
				Memo:            "",
				BlockHash:       block.BlockHash,
				BlockNumber:     block.BlockNumber,
				OutputIndex:     out.N,
				Confirmations:   tx.Confirmations,
				Amount:          amount,
			})
		}
	}
	return block, nil
}

func (chain *RPC) GetBlockByNumber(ctx context.Context, blockNumber int64) (*external.Block, error) {
	body, err := chain.call("getblockhash", []interface{}{blockNumber})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result string         `json:"result"`
		Error  *LitecoinError `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return chain.GetBlockByHash(ctx, resp.Result)
}

func (chain *RPC) GetTransactionConfirmations(transactionHash string) (int64, error) {
	tx, err := chain.getRawTransaction(transactionHash)
	if err == nil {
		return tx.Confirmations, nil
	}
	if berr, ok := err.(*LitecoinError); ok && berr.Code == -5 {
		return 0, nil
	}
	return 0, err
}

func (chain *RPC) SendRawTransaction(raw string) (string, error) {
	body, err := chain.call("sendrawtransaction", []interface{}{raw})
	if err != nil {
		return "", err
	}
	var resp struct {
		Result string         `json:"result"`
		Error  *LitecoinError `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", resp.Error
	}
	return resp.Result, nil
}

func (chain *RPC) getRawTransaction(txId string) (*LitecoinTransaction, error) {
	body, err := chain.call("getrawtransaction", []interface{}{txId, 1})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result LitecoinTransaction `json:"result"`
		Error  *LitecoinError      `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return &resp.Result, nil
}

type ScriptPubKey struct {
	Hex       string   `json:"hex"`
	Type      string   `json:"type"`
	Addresses []string `json:"addresses"`
}

type LitecoinIn struct {
	TxId string `json:"txid"`
	VOUT int64  `json:"vout"`
}

type LitecoinOut struct {
	Value        float64      `json:"value"`
	N            int64        `json:"n"`
	ScriptPubKey ScriptPubKey `json:"scriptPubKey"`
}

type LitecoinTransaction struct {
	TxId          string        `json:"txid"`
	Vin           []LitecoinIn  `json:"vin"`
	Vout          []LitecoinOut `json:"vout"`
	BlockHash     string        `json:"blockhash"`
	LockTime      int64         `json:"locktime"`
	Confirmations int64         `json:"confirmations"`
}

type LitecoinBlock struct {
	Hash   string   `json:"hash"`
	Height int64    `json:"height"`
	Tx     []string `json:"tx"`
}

type LitecoinError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (err *LitecoinError) Error() string {
	return fmt.Sprintf("BLOCK-API RPC ERROR Litecoin %d %s", err.Code, err.Message)
}

type ltcReq struct {
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      int64         `json:"id"`
	JSONRPC string        `json:"jsonrpc"`
}

func (chain *RPC) call(method string, params []interface{}) ([]byte, error) {
	data := ltcReq{
		Method:  method,
		Params:  params,
		Id:      time.Now().UnixNano(),
		JSONRPC: "2.0",
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("http://%s:%s@%s", chain.username, chain.password, chain.host)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Close = true
	req.Header.Set("Content-Type", "application/json")
	resp, err := chain.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
