package bsc

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/ethereum"
)

var (
	BinanceSmartChainBase string
	BinanceSmartChainId   crypto.Hash
)

func init() {
	BinanceSmartChainBase = "1949e683-6a08-49e2-b087-d6b72398588f"
	BinanceSmartChainId = crypto.NewHash([]byte(BinanceSmartChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if len(assetKey) != 42 {
		return fmt.Errorf("invalid binance smart asset key %s", assetKey)
	}
	if !strings.HasPrefix(assetKey, "0x") {
		return fmt.Errorf("invalid binance smart asset key %s", assetKey)
	}
	if assetKey != strings.ToLower(assetKey) {
		return fmt.Errorf("invalid binance smart asset key %s", assetKey)
	}
	k, err := hex.DecodeString(assetKey[2:])
	if err != nil {
		return fmt.Errorf("invalid binance smart asset key %s", assetKey)
	}
	if len(k) != 20 {
		return fmt.Errorf("invalid binance smart asset key %s", assetKey)
	}
	return nil
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid binance smart address %s", address)
	}
	if len(address) != 42 {
		return fmt.Errorf("invalid binance smart address %s", address)
	}
	if !strings.HasPrefix(address, "0x") {
		return fmt.Errorf("invalid binance smart address %s", address)
	}
	form, err := formatAddress(address)
	if err != nil {
		return fmt.Errorf("invalid binance smart address %s", address)
	}
	if form != address {
		return fmt.Errorf("invalid binance smart address %s", address)
	}
	a, err := hex.DecodeString(address[2:])
	if err != nil {
		return fmt.Errorf("invalid binance smart address %s %s", address, err.Error())
	}
	if len(a) != 20 {
		return fmt.Errorf("invalid binance smart address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 66 {
		return fmt.Errorf("invalid binance smart transaction hash %s", hash)
	}
	if !strings.HasPrefix(hash, "0x") {
		return fmt.Errorf("invalid binance smart transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid binance smart transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash[2:])
	if err != nil {
		return fmt.Errorf("invalid binance smart transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid binance smart transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	err := VerifyAssetKey(assetKey)
	if err != nil {
		panic(assetKey)
	}

	if assetKey == "0x0000000000000000000000000000000000000000" {
		return BinanceSmartChainId
	}

	return ethereum.BuildChainAssetId(BinanceSmartChainBase, assetKey)
}

func formatAddress(to string) (string, error) {
	var bytesto [20]byte
	_bytesto, err := hex.DecodeString(to[2:])
	if err != nil {
		return "", err
	}
	copy(bytesto[:], _bytesto)
	address := ethereum.Address(bytesto)
	return address.Hex(), nil
}
