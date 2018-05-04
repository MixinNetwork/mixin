package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"mixin.one/blockchain/external"
	"mixin.one/number"
)

const (
	rippleHost          = "s2.ripple.com:51234"
	rippleMinimumHeight = 37944529
)

type RPC struct {
	client *http.Client
	host   string
	id     string
}

type RippleLedger struct {
	LedgerIndex  string `json:"ledger_index"`
	LedgerHash   string `json:"ledger_hash"`
	Transactions []struct {
		TransactionType string      `json:"TransactionType"`
		Amount          interface{} `json:"Amount"`
		Hash            string      `json:"hash"`
		MetaData        struct {
			TransactionResult string `json:"TransactionResult"`
		} `json:"metaData"`
	} `json:"transactions"`
}

func NewRPC() (*RPC, error) {
	chain := &RPC{
		client: new(http.Client),
		host:   rippleHost,
		id:     external.RippleChainId,
	}
	height, err := chain.GetBlockHeight()
	if err != nil {
		return nil, err
	}
	if height < rippleMinimumHeight {
		return nil, fmt.Errorf("ripple block height too small %d", height)
	}
	return chain, nil
}

func (chain *RPC) GetBlockHeight() (int64, error) {
	body, err := chain.call("ledger", []interface{}{
		map[string]string{"ledger_index": "validated"},
	})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Result struct {
			Ledger RippleLedger `json:"ledger"`
			Status string       `json:"status"`
			Error  string       `json:"error"`
		} `json:"result"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return 0, err
	}
	result := resp.Result
	if result.Status != "success" {
		return 0, fmt.Errorf("ripple.GetBlockHeight response error %s %s", result.Status, result.Error)
	}
	ledgerIndex, err := strconv.ParseInt(result.Ledger.LedgerIndex, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("ripple.GetBlockHeight parse ledger index %s", err.Error())
	}
	if ledgerIndex < rippleMinimumHeight {
		return 0, fmt.Errorf("ripple.GetBlockHeight ledger index too small %d", ledgerIndex)
	}
	return ledgerIndex, nil
}

func (chain *RPC) EstimateFee() (number.Decimal, number.Decimal, error) {
	body, err := chain.call("server_info", []interface{}{map[string]string{}})
	if err != nil {
		return number.Zero(), number.Zero(), err
	}
	var resp struct {
		Result struct {
			Info struct {
				LoadFactor      float64 `json:"load_factor"`
				ValidatedLedger struct {
					BaseFeeXRP     float64 `json:"base_fee_xrp"`
					ReserveBaseXRP float64 `json:"reserve_base_xrp"`
				} `json:"validated_ledger"`
			} `json:"info"`
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"result"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return number.Zero(), number.Zero(), err
	}
	if resp.Result.Status != "success" {
		return number.Zero(), number.Zero(), fmt.Errorf("ripple.EstimateFee response error %s %s", resp.Result.Status, resp.Result.Error)
	}
	info := resp.Result.Info
	if info.LoadFactor <= 0 || info.ValidatedLedger.BaseFeeXRP <= 0 || info.ValidatedLedger.ReserveBaseXRP <= 0 {
		return number.Zero(), number.Zero(), fmt.Errorf("ripple.EstimateFee invalid %f %f %f", info.LoadFactor, info.ValidatedLedger.BaseFeeXRP, info.ValidatedLedger.ReserveBaseXRP)
	}
	fee := number.FromString(fmt.Sprint(info.LoadFactor)).Mul(number.FromString(fmt.Sprint(info.ValidatedLedger.BaseFeeXRP)))
	reserve := number.FromString(fmt.Sprint(info.ValidatedLedger.ReserveBaseXRP))
	if fee.Cmp(number.FromString("0.5")) < 0 {
		fee = number.FromString("0.5")
	}
	if reserve.Cmp(number.FromString("10")) < 0 {
		reserve = number.FromString("10")
	}
	return fee, reserve, nil
}

func (chain *RPC) GetBlock(ctx context.Context, id string) (*external.Block, error) {
	blockNumber, _ := strconv.ParseInt(id, 10, 64)
	if blockNumber > rippleMinimumHeight {
		return chain.GetBlockByNumber(ctx, blockNumber)
	}
	return chain.GetBlockByHash(ctx, id)
}

func (chain *RPC) GetBlockByHash(ctx context.Context, blockHash string) (*external.Block, error) {
	body, err := chain.call("ledger", []interface{}{
		map[string]interface{}{
			"ledger_hash":  blockHash,
			"transactions": true,
			"expand":       true,
		},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			Ledger    RippleLedger `json:"ledger"`
			Validated bool         `json:"validated"`
			Status    string       `json:"status"`
			Error     string       `json:"error"`
		} `json:"result"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	result := resp.Result
	if result.Status != "success" {
		return nil, fmt.Errorf("ripple.GetBlockByHash response error %s %s", result.Status, result.Error)
	}
	ledgerIndex, err := strconv.ParseInt(result.Ledger.LedgerIndex, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ripple.GetBlockByHash parse ledger index %s", err.Error())
	}
	if ledgerIndex < rippleMinimumHeight {
		return nil, fmt.Errorf("ripple.GetBlockByHash ledger index too small %d", ledgerIndex)
	}
	block := &external.Block{
		BlockHash:    result.Ledger.LedgerHash,
		BlockNumber:  ledgerIndex,
		Transactions: make([]*external.Transaction, 0),
	}
	if !result.Validated {
		return block, nil
	}
	for _, tx := range result.Ledger.Transactions {
		if tx.TransactionType != "Payment" {
			continue
		}
		if tx.MetaData.TransactionResult != "tesSUCCESS" {
			continue
		}
		amount := number.FromString(fmt.Sprint(tx.Amount))
		if amount.Exhausted() {
			continue
		}
		transaction, err := chain.getTransaction(tx.Hash)
		if err != nil {
			return nil, err
		}
		if transaction == nil {
			continue
		}
		if transaction.Amount.Exhausted() {
			continue
		}
		if transaction.Receipt != external.TransactionReceiptSuccessful {
			return nil, fmt.Errorf("ripple.GetBlockByHash invalid receipt %d", transaction.Receipt)
		}
		if transaction.Confirmations != 1 {
			return nil, fmt.Errorf("ripple.GetBlockByHash invalid confirmations %d", transaction.Confirmations)
		}
		if transaction.BlockNumber != block.BlockNumber {
			return nil, fmt.Errorf("ripple.GetBlockByHash invalid block number %d %d", transaction.BlockNumber, block.BlockNumber)
		}
		if transaction.TransactionHash != tx.Hash {
			return nil, fmt.Errorf("ripple.GetBlockByHash invalid transaction hash %s %s", transaction.TransactionHash, tx.Hash)
		}
		transaction.BlockHash = block.BlockHash
		block.Transactions = append(block.Transactions, transaction)
	}
	return block, nil
}

func (chain *RPC) GetBlockByNumber(ctx context.Context, blockNumber int64) (*external.Block, error) {
	body, err := chain.call("ledger", []interface{}{
		map[string]interface{}{"ledger_index": blockNumber},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			Ledger RippleLedger `json:"ledger"`
			Status string       `json:"status"`
			Error  string       `json:"error"`
		} `json:"result"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	result := resp.Result
	if result.Status != "success" {
		return nil, fmt.Errorf("ripple.GetBlockByNumber response error %s %s", result.Status, result.Error)
	}
	ledgerIndex, err := strconv.ParseInt(result.Ledger.LedgerIndex, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("ripple.GetBlockByNumber parse ledger index %s", err.Error())
	}
	if ledgerIndex < rippleMinimumHeight {
		return nil, fmt.Errorf("ripple.GetBlockByNumber ledger index too small %d", ledgerIndex)
	}
	return chain.GetBlockByHash(ctx, result.Ledger.LedgerHash)
}

func (chain *RPC) getTransaction(transactionHash string) (*external.Transaction, error) {
	body, err := chain.call("tx", []interface{}{
		map[string]interface{}{
			"transaction": transactionHash,
			"binary":      false,
		},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			TransactionType    string      `json:"TransactionType"`
			Account            string      `json:"Account"`
			Destination        string      `json:"Destination"`
			Amount             interface{} `json:"Amount"`
			Fee                string      `json:"Fee"`
			Hash               string      `json:"hash"`
			Sequence           int64       `json:"Sequence"`
			LastLedgerSequence int64       `json:"LastLedgerSequence"`
			LedgerIndex        int64       `json:"ledger_index"`
			LedgerHash         string      `json:"ledger_hash"`
			Meta               struct {
				DeliveredAmount   interface{} `json:"delivered_amount"`
				TransactionResult string      `json:"TransactionResult"`
			} `json:"meta"`
			Validated bool   `json:"validated"`
			Status    string `json:"status"`
			Error     string `json:"error"`
		} `json:"result"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	result := resp.Result
	if result.Status == "error" && result.Error == "txnNotFound" {
		return nil, nil
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("ripple.GetTransaction response error %s %s", result.Status, result.Error)
	}
	if result.LedgerIndex < rippleMinimumHeight {
		return nil, fmt.Errorf("ripple.GetTransaction ledger index too small %d", result.LedgerIndex)
	}
	amount := number.FromString(fmt.Sprint(result.Amount))
	deliveredAmount := number.FromString(fmt.Sprint(result.Meta.DeliveredAmount))
	if deliveredAmount.Cmp(amount) > 0 {
		return nil, fmt.Errorf("ripple.GetTransaction invalid delivered amount %s %s", amount.Persist(), deliveredAmount.Persist())
	}
	deliveredAmount = deliveredAmount.Mul(number.New(1, int32(6)))
	fee := number.FromString(result.Fee).Mul(number.New(1, int32(6)))
	if fee.Exhausted() {
		return nil, fmt.Errorf("ripple.GetTransaction invalid fee %s", fee.Persist())
	}
	asset := external.Asset{
		ChainId:       chain.id,
		AssetId:       chain.id,
		ChainAssetKey: chain.id,
		Symbol:        "XRP",
		Name:          "Ripple",
		Precision:     6,
	}
	confirmations, receipt := int64(0), external.TransactionReceiptSuccessful
	if result.Validated {
		confirmations = 1
		if result.Meta.TransactionResult != "tesSUCCESS" {
			receipt = external.TransactionReceiptFailed
		}
	}
	outputHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", result.Hash, 0)))
	return &external.Transaction{
		Asset:           asset,
		TransactionHash: result.Hash,
		Sender:          result.Account,
		Receiver:        result.Destination,
		Memo:            "",
		BlockNumber:     result.LedgerIndex,
		OutputIndex:     0,
		OutputHash:      hex.EncodeToString(outputHash[:]),
		Confirmations:   confirmations,
		Amount:          deliveredAmount,
		Fee:             fee,
		Receipt:         receipt,
	}, nil
}

func (chain *RPC) GetTransactionResult(transactionHash string) (*external.Transaction, error) {
	tx, err := chain.getTransaction(transactionHash)
	if err != nil || tx == nil {
		return &external.Transaction{
			Confirmations: 0,
			Fee:           number.Zero(),
			Receipt:       external.TransactionReceiptSuccessful,
		}, err
	}
	return tx, nil
}

func (chain *RPC) SendRawTransaction(raw string) (string, error) {
	body, err := chain.call("submit", []interface{}{
		map[string]string{"tx_blob": raw},
	})
	if err != nil {
		return "", err
	}
	var resp struct {
		Result struct {
			TxJSON struct {
				Hash string `json:"hash"`
			} `json:"tx_json"`
			EngineResult string `json:"engine_result"`
			Status       string `json:"status"`
			Error        string `json:"error"`
		} `json:"result"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return "", err
	}
	result := resp.Result
	if result.Status != "success" {
		return "", fmt.Errorf("ripple.SendRawTransaction response error %s %s", result.Status, result.Error)
	}
	if strings.HasPrefix(result.EngineResult, "tel") || strings.HasPrefix(result.EngineResult, "tem") ||
		strings.HasPrefix(result.EngineResult, "tef") || strings.HasPrefix(result.EngineResult, "ter") {
		return "", fmt.Errorf("ripple.SendRawTransaction response error %s %s %s", result.Status, result.Error, result.EngineResult)
	}
	return result.TxJSON.Hash, nil
}

type rippleReq struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

func (chain *RPC) call(method string, params []interface{}) ([]byte, error) {
	data := rippleReq{
		Method: method,
		Params: params,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	log.Println(string(body))

	endpoint := fmt.Sprintf("http://" + chain.host)
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ripple call %s error %d", method, resp.StatusCode)
	}
	return ioutil.ReadAll(resp.Body)
}
