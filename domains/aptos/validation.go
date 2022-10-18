package aptos

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	AptosAssetKey  string
	AptosChainBase string
	AptosChainId   crypto.Hash
)

func init() {
	AptosAssetKey = "0x1::aptos_coin::AptosCoin"
	AptosChainBase = "d2c1c7e1-a1a9-4f88-b282-d93b0a08b42b"
	AptosChainId = crypto.NewHash([]byte(AptosChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == AptosAssetKey {
		return nil
	}
	return fmt.Errorf("invalid aptos asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid aptos address %s", address)
	}
	if !strings.HasPrefix(address, "0x") {
		return fmt.Errorf("invalid aptos address prefix %s", address)
	}
	a, err := hex.DecodeString(strings.Replace(address, "0x", "", 1))
	if err != nil {
		return fmt.Errorf("invalid aptos address %s %s", address, err.Error())
	}
	if len(a) != 32 {
		return fmt.Errorf("invalid aptos address %s", address)
	}
	addr := fmt.Sprintf("0x%s", hex.EncodeToString(a))
	if addr != address {
		return fmt.Errorf("invalid aptos address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if !strings.HasPrefix(hash, "0x") {
		return fmt.Errorf("invalid aptos transaction hash prefix %s", hash)
	}
	h, err := hex.DecodeString(strings.Replace(hash, "0x", "", 1))
	if err != nil {
		return fmt.Errorf("invalid aptos transaction hash %s %s", hash, err.Error())
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid aptos transaction hash %s", hash)
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid aptos transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case AptosAssetKey:
		return AptosChainId
	default:
		panic(assetKey)
	}
}
