package bitcoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	BitcoinChainAssetKey    = "c6d0c728-2624-429b-8e0d-d9d19b6592fa"
	BitcoinOmniUSDTAssetKey = "815b0b1a-2764-3736-8faa-42d694fa620a"
)

var (
	BitcoinChainId    crypto.Hash
	BitcoinOmniUSDTId crypto.Hash
)

func init() {
	BitcoinChainId = crypto.NewHash([]byte(BitcoinChainAssetKey))
	BitcoinOmniUSDTId = crypto.NewHash([]byte(BitcoinOmniUSDTAssetKey))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == BitcoinChainAssetKey || assetKey == BitcoinOmniUSDTAssetKey {
		return nil
	}
	return fmt.Errorf("invalid bitcoin asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid bitcoin address %s", address)
	}
	err := DecodeCheckAddress(address)
	if err != nil {
		return fmt.Errorf("invalid bitcoin address %s %s", address, err.Error())
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid bitcoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid bitcoin transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid bitcoin transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid bitcoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case BitcoinChainAssetKey:
		return BitcoinChainId
	case BitcoinOmniUSDTAssetKey:
		return BitcoinOmniUSDTId
	default:
		panic(assetKey)
	}
}
