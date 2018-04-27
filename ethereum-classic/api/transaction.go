package api

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"mixin.one/number"
)

const (
	erc20TransferSignature = "0xa9059cbb"
	assetKeyEther          = "0x0000000000000000000000000000000000000000"
	assetPrecisionEther    = 18
)

var MainnetChainConfig = &params.ChainConfig{
	ChainId:             big.NewInt(61),
	HomesteadBlock:      big.NewInt(1150000),
	DAOForkBlock:        big.NewInt(1920000),
	DAOForkSupport:      true,
	EIP150Block:         big.NewInt(2463000),
	EIP150Hash:          common.HexToHash("0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0"),
	EIP155Block:         big.NewInt(2675000),
	EIP158Block:         big.NewInt(2675000),
	ByzantiumBlock:      big.NewInt(4370000),
	ConstantinopleBlock: nil,
	Ethash:              new(params.EthashConfig),
}

type Asset struct {
	Key       string
	Precision int32
}

func LocalNormalizePublicKey(address string) (string, error) {
	address = strings.TrimSpace(address)
	if len(address) != 42 {
		return "", fmt.Errorf("Ethereum invalid address %s", address)
	}
	_bytesto, err := hex.DecodeString(address[2:])
	if err != nil {
		return "", err
	}
	var bytesto [common.AddressLength]byte
	copy(bytesto[:], _bytesto)
	return common.Address(bytesto).Hex(), nil
}

func LocalSignRawTransaction(asset *Asset, receiver string, amount number.Decimal, gasPrice, gasLimit number.Decimal, privateKey string, nonce uint64) (string, string, error) {
	_, err := LocalNormalizePublicKey(receiver)
	if err != nil {
		return "", "", err
	}
	ethGasLimit := uint64(gasLimit.Float64())
	ethGasPrice, err := ethereumEtherToTokenPrecision(gasPrice, assetPrecisionEther)
	if err != nil {
		return "", "", err
	}
	ethAmount, err := ethereumEtherToTokenPrecision(amount, asset.Precision)
	if err != nil {
		return "", "", err
	}

	var data []byte
	if asset.Key != assetKeyEther {
		_, err := LocalNormalizePublicKey(asset.Key)
		if err != nil {
			return "", "", err
		}
		input, err := ethereumTokenTransfer(receiver, ethAmount)
		if err != nil {
			return "", "", err
		}
		data, err = hex.DecodeString(input[2:])
		if err != nil {
			return "", "", err
		}
		receiver = asset.Key
		ethAmount = big.NewInt(0)
	}
	return localSignRawTransaction(nonce, receiver, ethAmount, ethGasLimit, ethGasPrice, data, privateKey)
}

func localSignRawTransaction(nonce uint64, to string, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, privateKey string) (string, string, error) {
	to = strings.TrimSpace(to)
	if len(to) != 42 {
		return "", "", fmt.Errorf("invalid ethereum address %s", to)
	}

	ecdsaPriv, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return "", "", err
	}

	_bytesto, err := hex.DecodeString(to[2:])
	if err != nil {
		return "", "", err
	}
	var bytesto [common.AddressLength]byte
	copy(bytesto[:], _bytesto)
	address := common.Address(bytesto)
	tx := types.NewTransaction(nonce, address, amount, gasLimit, gasPrice, data)

	signer := types.MakeSigner(MainnetChainConfig, MainnetChainConfig.ByzantiumBlock)
	tx, err = types.SignTx(tx, signer, ecdsaPriv)
	if err != nil {
		return "", "", err
	}
	ts := types.Transactions{tx}
	txId := tx.Hash().Hex()
	raw := fmt.Sprintf("%x", ts.GetRlp(0))
	return txId, "0x" + raw, nil
}

func LocalGenerateKey() (string, string, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", "", err
	}
	address := crypto.PubkeyToAddress(key.PublicKey).Hex()
	private := hex.EncodeToString(crypto.FromECDSA(key))
	return address, private, nil
}

func ethereumEtherToTokenPrecision(ether number.Decimal, decimals int32) (*big.Int, error) {
	wei, success := new(big.Int).SetString(ether.Mul(number.New(1, -decimals)).Floor().String(), 0)
	if !success {
		return nil, fmt.Errorf("Ethereum failed to convert number %s", ether.String())
	}
	return wei, nil
}

func ethereumTokenTransfer(address string, wei *big.Int) (string, error) {
	address = address[2:len(address)]
	for len(address) < 64 {
		address = "0" + address
	}

	value := wei.Text(16)
	for len(value) < 64 {
		value = "0" + value
	}

	return erc20TransferSignature + address + value, nil
}
