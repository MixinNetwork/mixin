package bitcoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	BitcoinChainAssetKey = "c6d0c728-2624-429b-8e0d-d9d19b6592fa"
)

var (
	BitcoinChainId crypto.Hash
)

func init() {
	BitcoinChainId = crypto.Sha256Hash([]byte(BitcoinChainAssetKey))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == BitcoinChainAssetKey {
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
	default:
		panic(assetKey)
	}
}
