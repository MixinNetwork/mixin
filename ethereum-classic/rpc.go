package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"mixin.one/blockchain/external"
	"mixin.one/number"
)

// bash <(curl https://get.parity.io -Lk)
// nohup parity --chain classic --tracing on --cache-size 4096 --rpcapi eth,traces,parity --rpcaddr 0.0.0.0 --rpcport 8545 2>&1 > ~/.ethereum/parity.log &

const (
	ethereumMinimumHeight       = 1000000
	ethereumHost                = "ethereum-classic-full-node:8545"
	erc20TransferEventSignature = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	erc20DecimalsSignature      = "0x313ce567"
	erc20NameSignature          = "0x06fdde03"
	erc20SymbolSignature        = "0x95d89b41"
	erc20DECIMALSSignature      = "0x2e0f2625"
	erc20NAMESignature          = "0xa3f4df7e"
	erc20SYMBOLSignature        = "0xf76f8d78"
)

type RPC struct {
	client *http.Client
	host   string
	id     string
}

func NewRPC() (*RPC, error) {
	chain := &RPC{
		client: new(http.Client),
		host:   ethereumHost,
		id:     external.EthereumClassicChainId,
	}
	height, err := chain.GetBlockHeight()
	if err != nil {
		return nil, err
	}
	if height < ethereumMinimumHeight {
		return nil, fmt.Errorf("Ethereum block height too small %d", height)
	}
	return chain, nil
}

func (chain *RPC) GetBlockHeight() (int64, error) {
	body, err := chain.call("eth_blockNumber", []interface{}{})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Result EthereumQuantity `json:"result"`
		Error  *EthereumError   `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return 0, err
	}
	if resp.Error != nil {
		return 0, resp.Error
	}
	count, err := ethereumParseNumber(resp.Result)
	if err != nil {
		return 0, err
	}
	return count.Int64(), nil
}

func (chain *RPC) GetBlock(ctx context.Context, id string) (*external.Block, error) {
	blockNumber, _ := strconv.ParseInt(id, 10, 64)
	if blockNumber > ethereumMinimumHeight {
		return chain.GetBlockByNumber(ctx, blockNumber)
	}
	return chain.GetBlockByHash(ctx, id)
}

func (chain *RPC) GetBlockByHash(ctx context.Context, blockHash string) (*external.Block, error) {
	height, err := chain.GetBlockHeight()
	if err != nil {
		return nil, err
	}
	ethBlock, err := chain.getBlockByHash(blockHash)
	if err != nil {
		return nil, err
	}
	blockNumber, err := ethereumParseNumber(ethBlock.Number)
	if err != nil {
		return nil, err
	}
	return chain.parseEthereumBlock(ctx, ethBlock, blockNumber.Int64(), height)
}

func (chain *RPC) GetBlockByNumber(ctx context.Context, blockNumber int64) (*external.Block, error) {
	height, err := chain.GetBlockHeight()
	if err != nil {
		return nil, err
	}
	ethBlock, err := chain.getBlockByNumber(blockNumber)
	if err != nil {
		return nil, err
	}
	return chain.parseEthereumBlock(ctx, ethBlock, blockNumber, height)
}

func (chain *RPC) GetTransactionResult(transactionHash string) (*external.Transaction, error) {
	height, err := chain.GetBlockHeight()
	if err != nil {
		return nil, err
	}

	tx, err := chain.getTransactionByHash(transactionHash)
	if err != nil {
		return nil, err
	}
	if tx == nil {
		return &external.Transaction{
			Confirmations: 0,
			Fee:           number.Zero(),
			Receipt:       external.TransactionReceiptSuccessful,
		}, nil
	}
	blockNumber, err := ethereumParseNumber(tx.BlockNumber)
	if err != nil {
		return nil, err
	}
	if blockNumber.Int64() < ethereumMinimumHeight {
		return nil, fmt.Errorf("Ethereum block number too small %d", blockNumber)
	}
	bigGasPrice, err := ethereumParseNumber(tx.GasPrice)
	if err != nil {
		return nil, err
	}
	gasPrice := ethereumWeiToEther(bigGasPrice)

	receipt, err := chain.getTransactionReceipt(transactionHash)
	if err != nil {
		return nil, err
	}
	bigGasUsed, err := ethereumParseNumber(receipt.GasUsed)
	if err != nil {
		return nil, err
	}
	gasUsed := ethereumTokenPrecisionToHuman(bigGasUsed, 0)
	txReceipt := external.TransactionReceiptSuccessful

	return &external.Transaction{
		Confirmations: height - blockNumber.Int64(),
		Fee:           gasPrice.Mul(gasUsed),
		Receipt:       txReceipt,
	}, nil
}

func (chain *RPC) SendRawTransaction(raw string) (string, error) {
	body, err := chain.call("eth_sendRawTransaction", []interface{}{raw})
	if err != nil {
		return "", err
	}
	var resp struct {
		Result string         `json:"result"`
		Error  *EthereumError `json:"error,omitempty"`
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

func (chain *RPC) getTransactionByHash(hash string) (*EthereumTransaction, error) {
	body, err := chain.call("eth_getTransactionByHash", []interface{}{hash})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result *EthereumTransaction `json:"result,omitempty"`
		Error  *EthereumError       `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

func (chain *RPC) getTransactionReceipt(hash string) (*EthereumReceiptResult, error) {
	body, err := chain.call("eth_getTransactionReceipt", []interface{}{hash})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result EthereumReceiptResult `json:"result"`
		Error  *EthereumError        `json:"error,omitempty"`
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

func (chain *RPC) buildTree(tree *EthereumTraceTreeNode, trace *EthereumTraceResult) error {
	if len(trace.TraceAddress) == 1 {
		if int64(len(tree.SubTraces)) != trace.TraceAddress[0] {
			return fmt.Errorf("Ethereum traces tree malformed %d/%d %v", len(tree.SubTraces), trace.TraceAddress[0], trace)
		}
		node := &EthereumTraceTreeNode{EthereumTraceResult: trace}
		tree.SubTraces = append(tree.SubTraces, node)
		return nil
	} else {
		sub := tree.SubTraces[trace.TraceAddress[0]]
		trace.TraceAddress = trace.TraceAddress[1:]
		return chain.buildTree(sub, trace)
	}
}

func (chain *RPC) flattenTree(ctx context.Context, receipt *EthereumReceiptResult, height int64, transactions []*external.Transaction, node *EthereumTraceTreeNode) ([]*external.Transaction, error) {
	if node.Error != nil || node.Result == nil {
		return transactions, nil
	}

	transaction, err := chain.parseInternalTransaction(ctx, receipt, node.EthereumTraceResult, height, node.index)
	if err != nil {
		return transactions, err
	}
	if transaction != nil {
		transactions = append(transactions, transaction)
	}

	for _, sub := range node.SubTraces {
		subTransactions, err := chain.flattenTree(ctx, receipt, height, nil, sub)
		if err != nil {
			return transactions, err
		}
		transactions = append(transactions, subTransactions...)
	}
	return transactions, nil
}

func (chain *RPC) traverseTraces(ctx context.Context, receipt *EthereumReceiptResult, height int64, traces []*EthereumTraceResult) ([]*external.Transaction, error) {
	tree := &EthereumTraceTreeNode{EthereumTraceResult: traces[0]}
	for i, trace := range traces[1:] {
		trace.index = i + 1
		err := chain.buildTree(tree, trace)
		if err != nil {
			return nil, err
		}
	}
	return chain.flattenTree(ctx, receipt, height, nil, tree)
}

func (chain *RPC) parseEthereumBlock(ctx context.Context, ethBlock *EthereumBlock, blockNumber, height int64) (*external.Block, error) {
	block := &external.Block{
		BlockHash:    ethBlock.Hash,
		BlockNumber:  blockNumber,
		Transactions: make([]*external.Transaction, 0),
	}
	for _, txId := range ethBlock.Transactions {
		receipt, err := chain.getTransactionReceipt(txId)
		if err != nil {
			return nil, err
		}
		result, err := chain.traceTransaction(txId)
		if err != nil {
			return nil, err
		}
		if len(result) == 0 {
			continue
		}
		transactions, err := chain.traverseTraces(ctx, receipt, height, result)
		if err != nil {
			return nil, err
		}
		block.Transactions = append(block.Transactions, transactions...)
	}
	return block, nil
}

func (chain *RPC) getBlockByHash(blockHash string) (*EthereumBlock, error) {
	body, err := chain.call("eth_getBlockByHash", []interface{}{blockHash, false})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result EthereumBlock  `json:"result"`
		Error  *EthereumError `json:"error,omitempty"`
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

func (chain *RPC) getBlockByNumber(blockNumber int64) (*EthereumBlock, error) {
	body, err := chain.call("eth_getBlockByNumber", []interface{}{fmt.Sprintf("0x%x", blockNumber), false})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result EthereumBlock  `json:"result"`
		Error  *EthereumError `json:"error,omitempty"`
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

func (chain *RPC) traceTransaction(hash string) ([]*EthereumTraceResult, error) {
	body, err := chain.call("trace_transaction", []interface{}{hash})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result []*EthereumTraceResult `json:"result"`
		Error  *EthereumError         `json:"error,omitempty"`
	}
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

func (chain *RPC) parseInternalTransaction(ctx context.Context, receipt *EthereumReceiptResult, item *EthereumTraceResult, height int64, index int) (*external.Transaction, error) {
	if item.Error != nil || item.Result == nil {
		return nil, nil
	}
	if len(item.Action.To) == 0 {
		return nil, nil
	}
	if len(item.Action.To) != 42 {
		return nil, fmt.Errorf("Ethereum malformed action destination %s", item.Action.To)
	}
	if item.TransactionHash != receipt.TransactionHash {
		return nil, fmt.Errorf("Ethereum mismatched transaction hash %s %s", item.TransactionHash, receipt.TransactionHash)
	}

	asset := external.Asset{
		ChainId:       chain.id,
		AssetId:       chain.id,
		ChainAssetKey: "0x0000000000000000000000000000000000000000",
		Symbol:        "ETC",
		Name:          "Ether Classic",
		Precision:     18,
	}
	sender, receiver, amount := item.Action.From, item.Action.To, number.Zero()
	if token, err := chain.checkERC20Token(ctx, item.Action.To); err != nil {
		return nil, err
	} else if token != nil {
		asset.AssetId = token.Id
		asset.ChainAssetKey = token.Address
		asset.Symbol = token.Symbol
		asset.Name = token.Name
		asset.Precision = token.Decimals
		ev, es, er, err := chain.getTransferEvent(receipt, token, item.Action.Input)
		if err != nil {
			return nil, err
		}
		sender, receiver, amount = es, er, ev
	} else if item.Action.Value != "0x0" && item.Action.Value != "0x" && item.Action.Value != "" {
		if item.Result.Output != "0x" {
			return nil, nil
		}
		amountWei, err := ethereumParseNumber(item.Action.Value)
		if err != nil {
			return nil, err
		}
		amount = ethereumWeiToEther(amountWei)
	} else {
		return nil, nil
	}
	if amount.Exhausted() {
		return nil, nil
	}
	sender, err := chain.formatAddress(sender)
	if err != nil {
		return nil, err
	}
	receiver, err = chain.formatAddress(receiver)
	if err != nil {
		return nil, err
	}
	outputHash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", item.TransactionHash, index)))
	transaction := &external.Transaction{
		Asset:           asset,
		TransactionHash: item.TransactionHash,
		Sender:          sender,
		Receiver:        receiver,
		Memo:            "",
		BlockHash:       item.BlockHash,
		BlockNumber:     item.BlockNumber,
		OutputIndex:     int64(index),
		OutputHash:      hex.EncodeToString(outputHash[:]),
		Confirmations:   height - item.BlockNumber,
		Amount:          amount,
	}
	return transaction, nil
}

func (chain *RPC) getTransferEvent(receipt *EthereumReceiptResult, token *EthereumToken, input string) (number.Decimal, string, string, error) {
	for _, log := range receipt.Logs {
		if log.Address != token.Address {
			continue
		}
		if len(log.Topics) != 3 || len(input) < 66 {
			continue
		}
		if log.Topics[0] != erc20TransferEventSignature {
			continue
		}
		if !strings.Contains(input[10:], log.Topics[2][2:]) {
			continue
		}
		amountWei, err := ethereumParseNumber(EthereumQuantity(log.Data))
		if err != nil {
			return number.Zero(), "", "", err
		}
		amount := ethereumTokenPrecisionToHuman(amountWei, int(token.Decimals))
		address := "0x" + log.Topics[2][26:]
		sender := "0x" + log.Topics[1][26:]
		return amount, sender, address, nil
	}
	return number.Zero(), "", "", nil
}

func (chain *RPC) getTokenUpperSymbolAndName(ctx context.Context, address string) (string, string, error) {
	erc20, err := abi.JSON(strings.NewReader(erc20UpperABI))
	if err != nil {
		return "", "", err
	}
	erc20Alt, err := abi.JSON(strings.NewReader(erc20UpperABIAlt))
	if err != nil {
		return "", "", err
	}

	result, err := chain.ethereumCall(address, erc20SYMBOLSignature)
	if err != nil || len(result) < 66 {
		return "", "", err
	}
	resultBytes, err := hex.DecodeString(result[2:])
	if err != nil {
		return "", "", err
	}
	var symbol string
	if len(resultBytes) == 32 {
		var bytes32 [32]byte
		err = erc20Alt.Unpack(&bytes32, "SYMBOL", resultBytes)
		if err != nil {
			return "", "", nil
		}
		resultBytes = bytes.Trim(bytes32[:], "\x00")
		if utf8.Valid(resultBytes) {
			symbol = string(resultBytes)
		}
	} else {
		err = erc20.Unpack(&symbol, "SYMBOL", resultBytes)
		if err != nil {
			return "", "", nil
		}
	}

	result, err = chain.ethereumCall(address, erc20NAMESignature)
	if err != nil || len(result) < 66 {
		return "", "", err
	}
	resultBytes, err = hex.DecodeString(result[2:])
	if err != nil {
		return "", "", err
	}
	var name string
	if len(resultBytes) == 32 {
		var bytes32 [32]byte
		err = erc20Alt.Unpack(&bytes32, "NAME", resultBytes)
		if err != nil {
			return "", "", nil
		}
		resultBytes = bytes.Trim(bytes32[:], "\x00")
		if utf8.Valid(resultBytes) {
			name = string(resultBytes)
		}
	} else {
		err = erc20.Unpack(&name, "NAME", resultBytes)
		if err != nil {
			return "", "", nil
		}
	}

	return symbol, name, nil
}

func (chain *RPC) getTokenSymbolAndName(ctx context.Context, address string) (string, string, error) {
	erc20, err := abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		return "", "", err
	}
	erc20Alt, err := abi.JSON(strings.NewReader(erc20ABIAlt))
	if err != nil {
		return "", "", err
	}

	result, err := chain.ethereumCall(address, erc20SymbolSignature)
	if err != nil || len(result) < 66 {
		return "", "", err
	}
	resultBytes, err := hex.DecodeString(result[2:])
	if err != nil {
		return "", "", err
	}
	var symbol string
	if len(resultBytes) == 32 {
		var bytes32 [32]byte
		err = erc20Alt.Unpack(&bytes32, "symbol", resultBytes)
		if err != nil {
			return "", "", nil
		}
		resultBytes = bytes.Trim(bytes32[:], "\x00")
		if utf8.Valid(resultBytes) {
			symbol = string(resultBytes)
		}
	} else {
		err = erc20.Unpack(&symbol, "symbol", resultBytes)
		if err != nil {
			return "", "", nil
		}
	}

	result, err = chain.ethereumCall(address, erc20NameSignature)
	if err != nil || len(result) < 66 {
		return "", "", err
	}
	resultBytes, err = hex.DecodeString(result[2:])
	if err != nil {
		return "", "", err
	}
	var name string
	if len(resultBytes) == 32 {
		var bytes32 [32]byte
		err = erc20Alt.Unpack(&bytes32, "name", resultBytes)
		if err != nil {
			return "", "", nil
		}
		resultBytes = bytes.Trim(bytes32[:], "\x00")
		if utf8.Valid(resultBytes) {
			name = string(resultBytes)
		}
	} else {
		err = erc20.Unpack(&name, "name", resultBytes)
		if err != nil {
			return "", "", nil
		}
	}

	return symbol, name, nil
}

func (chain *RPC) getTokenSymbolAndNameAndDecimals(ctx context.Context, address string) (string, string, int64, error) {
	symbol, name, err := chain.getTokenSymbolAndName(ctx, address)
	if err != nil {
		return "", "", 0, err
	}

	result, err := chain.ethereumCall(address, erc20DecimalsSignature)
	if err != nil || len(result) != 66 {
		return "", "", 0, err
	}
	bigDecimals, err := ethereumParseNumber(EthereumQuantity(result))
	if err != nil {
		return "", "", 0, err
	}
	var decimals = bigDecimals.Int64()
	return symbol, name, decimals, nil
}

func (chain *RPC) getTokenUpperSymbolAndNameAndDecimals(ctx context.Context, address string) (string, string, int64, error) {
	symbol, name, err := chain.getTokenUpperSymbolAndName(ctx, address)
	if err != nil {
		return "", "", 0, err
	}

	result, err := chain.ethereumCall(address, erc20DECIMALSSignature)
	if err != nil || len(result) != 66 {
		return "", "", 0, err
	}
	bigDecimals, err := ethereumParseNumber(EthereumQuantity(result))
	if err != nil {
		return "", "", 0, err
	}
	var decimals = bigDecimals.Int64()
	return symbol, name, decimals, nil
}

func (chain *RPC) checkERC20Token(ctx context.Context, address string) (*EthereumToken, error) {
	address = strings.ToLower(strings.TrimSpace(address))
	assetId := external.UniqueAssetId(chain.id, address)
	if token, err := persistReadToken(ctx, assetId); err != nil {
		return nil, err
	} else if token != nil {
		if token.Chain != chain.id || token.Address != address {
			return nil, fmt.Errorf("Ethereum malformed token persistence %s|%s => %s|%s", chain.id, address, token.Chain, token.Address)
		}
		return token, nil
	}

	symbol, name, decimals, err := chain.getTokenSymbolAndNameAndDecimals(ctx, address)
	if err != nil {
		ethErr, ok := err.(*EthereumError)
		if !ok || ethErr.Code != -32015 {
			return nil, err
		}
	}
	if len(symbol) < 2 {
		symbol, name, decimals, err = chain.getTokenUpperSymbolAndNameAndDecimals(ctx, address)
	}
	if err != nil {
		ethErr, ok := err.(*EthereumError)
		if ok && ethErr.Code == -32015 {
			return nil, nil
		}
		return nil, err
	}
	if len(symbol) < 2 {
		return nil, nil
	}

	token := &EthereumToken{
		Chain:    chain.id,
		Id:       assetId,
		Address:  address,
		Symbol:   symbol,
		Name:     name,
		Decimals: decimals,
	}
	return token, persistWriteToken(ctx, token)
}

func (chain *RPC) ethereumCall(to, data string) (string, error) {
	params := map[string]interface{}{"to": to, "data": data}
	body, err := chain.call("eth_call", []interface{}{params})
	if err != nil {
		return "", err
	}
	var resp struct {
		Result string         `json:"result"`
		Error  *EthereumError `json:"error,omitempty"`
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

func ethereumParseNumber(hex EthereumQuantity) (*big.Int, error) {
	value, success := new(big.Int).SetString(string(hex), 0)
	if !success {
		return nil, fmt.Errorf("Ethereum failed to parse quantity %s", string(hex))
	}
	return value, nil
}

func ethereumWeiToEther(wei *big.Int) number.Decimal {
	return ethereumTokenPrecisionToHuman(wei, 18)
}

func ethereumTokenPrecisionToHuman(value *big.Int, decimals int) number.Decimal {
	return number.FromString(value.Text(10)).Mul(number.New(1, int32(decimals)))
}

type EthereumToken struct {
	Chain    string
	Id       string
	Name     string
	Symbol   string
	Address  string
	Decimals int64
}

type EthereumTransaction struct {
	Hash        string           `json:"hash"`
	From        string           `json:"from"`
	To          string           `json:"to"`
	Input       string           `json:"input"`
	Value       EthereumQuantity `json:"value"`
	BlockNumber EthereumQuantity `json:"blockNumber"`
	BlockHash   string           `json:"blockHash"`
	GasPrice    EthereumQuantity `json:"gasPrice"`
}

type EthereumReceiptResult struct {
	BlockNumber     EthereumQuantity `json:"blockNumber"`
	TransactionHash string           `json:"transactionHash"`
	ContractAddress string           `json:"contractAddress"`
	Logs            []struct {
		Address string   `json:"address"`
		Data    string   `json:"data"`
		Topics  []string `json:"topics"`
	} `json:"logs"`
	GasUsed EthereumQuantity `json:"gasUsed"`
}

type EthereumTraceResultOutput struct {
	GasUsed EthereumQuantity `json:"gasUsed"`
	Output  string           `json:"output"`
}

type EthereumTraceResult struct {
	Action struct {
		From  string           `json:"from"`
		To    string           `json:"to"`
		Input string           `json:"input"`
		Value EthereumQuantity `json:"value"`
	} `json:"action"`
	BlockHash       string                     `json:"blockHash"`
	BlockNumber     int64                      `json:"blockNumber"`
	TransactionHash string                     `json:"transactionHash"`
	Error           interface{}                `json:"error,omitempty"`
	Result          *EthereumTraceResultOutput `json:"result,omitempty"`
	SubTraces       int64                      `json:"subtraces"`
	TraceAddress    []int64                    `json:"traceAddress"`
	index           int
}

type EthereumTraceTreeNode struct {
	*EthereumTraceResult
	SubTraces []*EthereumTraceTreeNode
}

type EthereumBlock struct {
	Number       EthereumQuantity `json:"number"`
	Hash         string           `json:"hash"`
	Transactions []string         `json:"transactions"`
}

type EthereumError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (err *EthereumError) Error() string {
	return fmt.Sprintf("BLOCK-API RPC ERROR Ethereum %d %s", err.Code, err.Message)
}

type EthereumQuantity = string

type ethReq struct {
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      int64         `json:"id"`
	JSONRPC string        `json:"jsonrpc"`
}

func (chain *RPC) call(method string, params []interface{}) ([]byte, error) {
	data := ethReq{
		Method:  method,
		Params:  params,
		Id:      time.Now().UnixNano(),
		JSONRPC: "2.0",
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "http://"+chain.host, bytes.NewReader(body))
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

func (chain *RPC) formatAddress(to string) (string, error) {
	var bytesto [20]byte
	_bytesto, err := hex.DecodeString(to[2:])
	if err != nil {
		return "", err
	}
	copy(bytesto[:], _bytesto)
	address := common.Address(bytesto)
	return address.Hex(), nil
}
