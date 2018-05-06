package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/NebulousLabs/Sia/encoding"
	"github.com/NebulousLabs/Sia/types"
	"mixin.one/blockchain/external"
	"mixin.one/number"
)

// nohup siad -M gcte -d /home/one/.siad &

const (
	siacoinMinimumHeight = 100000
	siacoinHost          = "localhost:9980"
)

type RPC struct {
	client *http.Client
	host   string
	id     string
}

func NewRPC() (*RPC, error) {
	chain := &RPC{
		client: new(http.Client),
		host:   siacoinHost,
		id:     external.SiacoinChainId,
	}
	height, err := chain.GetBlockHeight()
	if err != nil {
		return nil, err
	}
	if height < siacoinMinimumHeight {
		return nil, fmt.Errorf("Siacoin block height too small %d", height)
	}
	return chain, nil
}

func (chain *RPC) GetBlockHeight() (int64, error) {
	body, err := chain.call("GET", "/consensus")
	if err != nil {
		return 0, err
	}
	var resp struct {
		Message string `json:"message"`
		Height  int64  `json:"height"`
		Synced  bool   `json:"synced"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return 0, err
	}
	if len(resp.Message) > 0 {
		return 0, fmt.Errorf("Siacoin GetBlockHeight error %s", resp.Message)
	}
	if !resp.Synced {
		return 0, fmt.Errorf("Siacoin not synced yet %d", resp.Height)
	}
	if resp.Height < siacoinMinimumHeight {
		return 0, fmt.Errorf("Siacoin block height too small %d", resp.Height)
	}
	return resp.Height, nil
}

func (chain *RPC) EstimateSmartFee() (number.Decimal, error) {
	body, err := chain.call("GET", "/tpool/fee")
	if err != nil {
		return number.Zero(), err
	}
	var resp struct {
		Message string `json:"message"`
		Minimum string `json:"minimum"`
		Maximum string `json:"maximum"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return number.Zero(), err
	}
	if len(resp.Message) > 0 {
		return number.Zero(), fmt.Errorf("Siacoin EstimateSmartFee error %s", resp.Message)
	}
	min := number.FromString("0.3")
	fee := number.FromString(resp.Maximum).Mul(number.New(1, 24)).Mul(number.FromString("0.3"))
	if fee.Exhausted() {
		return number.Zero(), fmt.Errorf("Siacoin EstimateSmartFee invalid %s %s", resp.Minimum, resp.Maximum)
	}
	if fee.Cmp(min) < 0 {
		fee = min
	}
	return fee.Round(8), nil
}

func (chain *RPC) GetBlock(ctx context.Context, id string) (*external.Block, error) {
	blockNumber, _ := strconv.ParseInt(id, 10, 64)
	if blockNumber > siacoinMinimumHeight {
		return chain.GetBlockByNumber(ctx, blockNumber)
	}
	return chain.GetBlockByHash(ctx, id)
}

func (chain *RPC) GetBlockByHash(ctx context.Context, blockHash string) (*external.Block, error) {
	body, err := chain.call("GET", "/explorer/hashes/"+blockHash)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Message  string `json:"message"`
		HashType string `json:"hashtype"`
		Block    struct {
			BlockId string `json:"blockid"`
			Height  int64  `json:"height"`
		} `json:"block"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Message) > 0 {
		return nil, fmt.Errorf("Siacoin GetBlockByHash error %s", resp.Message)
	}
	if resp.HashType != "blockid" || resp.Block.BlockId != blockHash {
		return nil, fmt.Errorf("Siacoin GetBlockByHash malformed %s %s %s", blockHash, resp.HashType, resp.Block.BlockId)
	}
	if resp.Block.Height < siacoinMinimumHeight {
		return nil, fmt.Errorf("Siacoin GetBlockByHash height too small %d", resp.Block.Height)
	}
	return chain.GetBlockByNumber(ctx, resp.Block.Height)
}

func (chain *RPC) GetBlockByNumber(ctx context.Context, blockNumber int64) (*external.Block, error) {
	body, err := chain.call("GET", fmt.Sprintf("/explorer/blocks/%d", blockNumber))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Message string `json:"message"`
		Block   struct {
			Transactions []struct {
				Id               string            `json:"id"`
				Height           int64             `json:"height"`
				Parent           string            `json:"parent"`
				RawTransaction   types.Transaction `json:"rawtransaction"`
				SiacoinOutputIds []string          `json:"siacoinoutputids"`
			} `json:"transactions"`
		} `json:"block"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Message) > 0 {
		return nil, fmt.Errorf("Siacoin GetBlockByNumber error %s", resp.Message)
	}
	height, err := chain.GetBlockHeight()
	if err != nil {
		return nil, err
	}
	asset := external.Asset{
		ChainId:       chain.id,
		AssetId:       chain.id,
		ChainAssetKey: "siacoin",
		Symbol:        "SC",
		Name:          "Siacoin",
		Precision:     24,
	}
	block := &external.Block{
		BlockNumber:  blockNumber,
		Transactions: make([]*external.Transaction, 0),
	}
	for _, tx := range resp.Block.Transactions {
		if tx.Parent == "" {
			return nil, fmt.Errorf("Siacoin GetBlockByNumber invalid block hash %d", blockNumber)
		}
		if block.BlockHash == "" {
			block.BlockHash = tx.Parent
		}
		if height < tx.Height {
			return nil, fmt.Errorf("Siacoin GetBlockByNumber height in the future %d %d", tx.Height, height)
		}
		if tx.Height != blockNumber {
			return nil, fmt.Errorf("Siacoin GetBlockByNumber malformed %d %d", blockNumber, tx.Height)
		}
		if tx.Height < siacoinMinimumHeight {
			return nil, fmt.Errorf("Siacoin GetBlockByNumber height too small %d", tx.Height)
		}
		if tx.Parent != block.BlockHash {
			return nil, fmt.Errorf("Siacoin GetBlockByNumber malformed transaction %s %s", tx.Parent, block.BlockHash)
		}
		if len(tx.RawTransaction.SiacoinOutputs) != len(tx.SiacoinOutputIds) {
			return nil, fmt.Errorf("Siacoin GetBlockByNumber malformed outputs %d %d", len(tx.RawTransaction.SiacoinOutputs), len(tx.SiacoinOutputIds))
		}
		for index, output := range tx.RawTransaction.SiacoinOutputs {
			amount := number.FromString(output.Value.String()).Mul(number.New(1, 24))
			if amount.Exhausted() {
				continue
			}
			outputIndex := int64(index)
			outputId := tx.RawTransaction.SiacoinOutputID(uint64(outputIndex)).String()
			if outputId != tx.SiacoinOutputIds[index] {
				return nil, fmt.Errorf("Siacoin GetBlockByNumber malformed output id %s %s", outputId, tx.SiacoinOutputIds[index])
			}
			block.Transactions = append(block.Transactions, &external.Transaction{
				Asset:           asset,
				TransactionHash: tx.Id,
				Sender:          "",
				Receiver:        output.UnlockHash.String(),
				Memo:            "",
				BlockHash:       block.BlockHash,
				BlockNumber:     block.BlockNumber,
				OutputIndex:     outputIndex,
				OutputHash:      outputId,
				Confirmations:   height - block.BlockNumber,
				Amount:          amount,
			})
		}
	}
	return block, nil
}

func (chain *RPC) GetTransactionConfirmations(transactionHash string) (int64, error) {
	body, err := chain.call("GET", "/explorer/hashes/"+transactionHash)
	if err != nil {
		if strings.Contains(err.Error(), "hash not found in hashtype db") {
			return 0, nil
		}
		return 0, err
	}
	var resp struct {
		Message     string `json:"message"`
		HashType    string `json:"hashtype"`
		Transaction struct {
			Id     string `json:"id"`
			Height int64  `json:"height"`
		} `json:"transaction"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return 0, err
	}
	if len(resp.Message) > 0 {
		return 0, fmt.Errorf("Siacoin GetTransactionConfirmations error %s", resp.Message)
	}
	if resp.HashType != "transactionid" || resp.Transaction.Id != transactionHash {
		return 0, fmt.Errorf("Siacoin GetTransactionConfirmations malformed %s %s %s", transactionHash, resp.HashType, resp.Transaction.Id)
	}
	if resp.Transaction.Height < siacoinMinimumHeight {
		return 0, fmt.Errorf("Siacoin GetTransactionConfirmations height too small %d", resp.Transaction.Height)
	}
	height, err := chain.GetBlockHeight()
	if err != nil {
		return 0, err
	}
	if height < resp.Transaction.Height {
		return 0, fmt.Errorf("Siacoin GetTransactionConfirmations height in the future %d %d", resp.Transaction.Height, height)
	}
	return height - resp.Transaction.Height, nil
}

func (chain *RPC) SendRawTransaction(raw string) (string, error) {
	rawBytes, err := hex.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("Siacoin SendRawTransaction invalid raw %s", raw)
	}

	var tx types.Transaction
	err = encoding.Unmarshal(rawBytes, &tx)
	if err != nil {
		return "", fmt.Errorf("Siacoin SendRawTransaction invalid transaction %s", raw)
	}

	values := url.Values{}
	values.Set("transaction", string(rawBytes))
	values.Set("parents", string(encoding.Marshal([]types.Transaction{})))

	endpoint := "/tpool/raw?" + values.Encode()
	_, err = chain.call("POST", endpoint)
	if err != nil {
		return "", fmt.Errorf("Siacoin SendRawTransaction call error %s", err.Error())
	}
	return tx.ID().String(), nil
}

func (chain *RPC) call(method, path string) ([]byte, error) {
	endpoint := "http://" + chain.host + path
	if strings.HasPrefix(path, "/explorer/hashes") {
		endpoint = "https://explore.sia.tech/api/hashes" + path[16:]
	}
	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Close = true
	req.Header.Set("User-Agent", "Sia-Agent")
	resp, err := chain.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Siacoin call body error %s %s %d %s", method, path, resp.StatusCode, err.Error())
	}
	if resp.StatusCode < 200 || resp.StatusCode > 300 {
		return nil, fmt.Errorf("Siacoin call status error %s %s %d %s", method, path, resp.StatusCode, string(body))
	}
	return body, nil
}
