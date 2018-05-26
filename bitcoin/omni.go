package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"mixin.one/blockchain/external"
	"mixin.one/number"
)

var omniClient = new(http.Client)

const (
	omniHost     = "bitcoin-full-node:9332"
	omniUsername = "2deca196257ec90d2aca14acffe25014"
	omniPassword = "f83de9a2f5ef56221db2c529f525f15e2d2e2e9cd2a02d5adc2c9a97c7aff1a8"
)

type OmniTransaction struct {
	TxId             string `json:"txid"`
	Confirmations    int64  `json:"confirmations"`
	SendingAddress   string `json:"sendingaddress"`
	ReferenceAddress string `json:"referenceaddress"`
	Type             string `json:"type"`
	TypeInt          int64  `json:"type_int"`
	Version          int64  `json:"version"`
	Valid            bool   `json:"valid"`
	PropertyId       int64  `json:"propertyid"`
	Amount           string `json:"amount"`
}

func omniGetTransaction(block *external.Block, txId string, outputIndex int64) (*external.Transaction, error) {
	omniHeight, err := omniBlockHeight()
	if err != nil {
		return nil, err
	}
	if omniHeight < block.BlockNumber {
		return nil, fmt.Errorf("Omni block height too small %d %d %s", omniHeight, block.BlockNumber, txId)
	}

	body, err := omniCall("omni_gettransaction", []interface{}{txId})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result OmniTransaction `json:"result"`
		Error  *BitcoinError   `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		if resp.Error.Message == "Not a Master Protocol transaction" {
			return nil, nil
		}
		return nil, resp.Error
	}
	tx := &resp.Result
	if !tx.Valid || tx.Version != 0 || tx.TypeInt != 0 || tx.Type != "Simple Send" || tx.TxId != txId || tx.PropertyId != 31 {
		return nil, nil
	}
	amount := number.FromString(tx.Amount)
	if amount.Cmp(number.FromString("1")) < 0 {
		return nil, nil
	}
	asset := external.Asset{
		ChainId:       external.BitcoinChainId,
		AssetId:       external.BitcoinOmniUSDTId,
		ChainAssetKey: external.BitcoinOmniUSDTId,
		Symbol:        "USDT",
		Name:          "Tether USD",
		Precision:     8,
	}
	outputHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", tx.TxId, outputIndex)))
	return &external.Transaction{
		Asset:           asset,
		TransactionHash: tx.TxId,
		Sender:          tx.SendingAddress,
		Receiver:        tx.ReferenceAddress,
		Memo:            "",
		BlockHash:       block.BlockHash,
		BlockNumber:     block.BlockNumber,
		OutputIndex:     outputIndex,
		OutputHash:      hex.EncodeToString(outputHash[:]),
		Confirmations:   tx.Confirmations,
		Amount:          amount,
	}, nil
}

func omniBlockHeight() (int64, error) {
	body, err := omniCall("getblockchaininfo", []interface{}{})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Result struct {
			Blocks int64 `json:"blocks"`
		} `json:"result"`
		Error *BitcoinError `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return 0, err
	}
	if resp.Error != nil {
		return 0, resp.Error
	}
	if resp.Result.Blocks < bitcoinMinimumHeight {
		return 0, fmt.Errorf("Bitcoin block height too small %d", resp.Result.Blocks)
	}
	return resp.Result.Blocks, nil
}

func omniCall(method string, params []interface{}) ([]byte, error) {
	data := btcReq{
		Method:  method,
		Params:  params,
		Id:      time.Now().UnixNano(),
		JSONRPC: "2.0",
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("http://%s:%s@%s", omniUsername, omniPassword, omniHost)
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Close = true
	req.Header.Set("Content-Type", "application/json")
	resp, err := omniClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}
