package ton

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	TonAssetKey  string
	TonChainBase string
	TonChainId   crypto.Hash
)

func init() {
	TonAssetKey = "ef660437-d915-4e27-ad3f-632bfb6ba0ee"
	TonChainBase = "ef660437-d915-4e27-ad3f-632bfb6ba0ee"
	TonChainId = crypto.NewHash([]byte(TonChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == TonAssetKey {
		return nil
	}
	return fmt.Errorf("invalid ton asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid ton address %s", address)
	}
	data, err := base64.RawURLEncoding.DecodeString(address)
	if err != nil {
		return fmt.Errorf("invalid ton decode address %s", address)
	}
	if len(data) != 36 {
		return fmt.Errorf("invalid ton address %s", address)
	}
	addr := base64.RawURLEncoding.EncodeToString(data)
	if addr != address {
		return fmt.Errorf("invalid ton address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	h, err := base64.URLEncoding.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid ton transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid ton transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case TonAssetKey:
		return TonChainId
	default:
		panic(assetKey)
	}
}
