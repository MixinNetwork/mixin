package external

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"mixin.one/number"
	"mixin.one/uuid"
)

const privateKey = ""

var client = &http.Client{}

func SendRawTransaction(chainId string, raw string) (string, error) {
	content, err := json.Marshal(map[string]string{"raw": raw})
	if err != nil {
		return "", err
	}
	resp, err := getChainRequest(chainId, "POST", "/transactions", bytes.NewReader(content))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var body struct {
		Data struct {
			TransactionHash string `json:"transaction_hash"`
		} `json:"data"`
		Error interface{} `json:"error,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return "", err
	}
	if body.Error != nil {
		return "", fmt.Errorf("connect API error %s", body.Error)
	}
	return body.Data.TransactionHash, nil
}

func GetBlockHeight(chainId string) (int64, error) {
	resp, err := getChainRequest(chainId, "GET", "/height", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var body struct {
		Data struct {
			Height int64 `json:"height"`
		} `json:"data"`
		Error interface{} `json:"error,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return 0, err
	}
	if body.Error != nil {
		return 0, fmt.Errorf("connect API error %s", body.Error)
	}
	return body.Data.Height, nil
}

func GetBlock(chainId string, blockNumber int64) (*Block, error) {
	resp, err := getChainRequest(chainId, "GET", fmt.Sprintf("/blocks/%d", blockNumber), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Data  Block       `json:"data"`
		Error interface{} `json:"error,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return nil, err
	}
	if body.Error != nil {
		return nil, fmt.Errorf("connect API error %s", body.Error)
	}
	return &body.Data, nil
}

func GetAsset(chainId string, assetId string) (*Asset, error) {
	resp, err := getChainRequest(chainId, "GET", fmt.Sprintf("/assets/%s", assetId), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Data  Asset       `json:"data"`
		Error interface{} `json:"error,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return nil, err
	}
	if body.Error != nil {
		return nil, fmt.Errorf("connect API error %s", body.Error)
	}
	return &body.Data, nil
}

func GetTransactionResult(chainId string, transactionHash string) (*Transaction, error) {
	resp, err := getChainRequest(chainId, "GET", fmt.Sprintf("/transactions/%s", transactionHash), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body struct {
		Data  Transaction `json:"data"`
		Error interface{} `json:"error,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return nil, err
	}
	if body.Error != nil {
		return nil, fmt.Errorf("connect API error %s", body.Error)
	}
	return &body.Data, nil
}

func GetEstimatedFee(chainId string, blockCount int64) (number.Decimal, number.Decimal, error) {
	resp, err := getChainRequest(chainId, "GET", "/fee", nil)
	if err != nil {
		return number.Zero(), number.Zero(), err
	}
	defer resp.Body.Close()
	var body struct {
		Data struct {
			FeePerKb     string `json:"fee_per_kb"`
			GasPrice     string `json:"gas_price"`
			GasLimit     string `json:"gas_limit"`
			FeeInXRP     string `json:"fee_in_xrp"`
			ReserveInXRP string `json:"reserve_in_xrp"`
		} `json:"data"`
		Error interface{} `json:"error,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return number.Zero(), number.Zero(), err
	}
	if body.Error != nil {
		return number.Zero(), number.Zero(), fmt.Errorf("connect API error %s", body.Error)
	}
	switch chainId {
	case BitcoinChainId, BitcoinCashChainId, LitecoinChainId:
		return number.FromString(body.Data.FeePerKb), number.Zero(), nil
	case EthereumChainId, EthereumClassicChainId:
		return number.FromString(body.Data.GasPrice), number.FromString(body.Data.GasLimit), nil
	case RippleChainId:
		return number.FromString(body.Data.FeeInXRP), number.FromString(body.Data.ReserveInXRP), nil
	case SiacoinChainId:
		return number.FromString(body.Data.FeePerKb), number.Zero(), nil
	}
	return number.Zero(), number.Zero(), fmt.Errorf("unsupported chain %s", chainId)
}

func getChainRequest(chainId string, method, path string, body io.Reader) (*http.Response, error) {
	var endpoint string
	switch chainId {
	case RippleChainId:
		endpoint = "https://ripple.connect.mixin.one" + path
	case SiacoinChainId:
		endpoint = "https://siacoin.connect.mixin.one" + path
	case EthereumChainId:
		endpoint = "https://ethereum.connect.mixin.one" + path
	case EthereumClassicChainId:
		endpoint = "https://ethereum-classic.connect.mixin.one" + path
	case BitcoinChainId:
		endpoint = "https://bitcoin.connect.mixin.one" + path
	case BitcoinCashChainId:
		endpoint = "https://bitcoin-cash.connect.mixin.one" + path
	case LitecoinChainId:
		endpoint = "https://litecoin.connect.mixin.one" + path
	default:
		return nil, fmt.Errorf("unsupported chain id %s", chainId)
	}
	token, err := signAuthenticationToken()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

func signAuthenticationToken() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, jwt.MapClaims{
		"exp": time.Now().Add(time.Minute * 3).Unix(),
		"jti": uuid.NewV4().String(),
	})

	block, _ := pem.Decode([]byte(privateKey))
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	return token.SignedString(key)
}
