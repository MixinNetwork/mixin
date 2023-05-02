package sui

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	SuiAssetKey  string
	SuiChainBase string
	SuiChainId   crypto.Hash
)

func init() {
	SuiAssetKey = "0x2::sui::SUI"
	SuiChainBase = "b1ad4729-2c39-4e7e-8bd6-c63c21941a0e"
	SuiChainId = crypto.NewHash([]byte(SuiChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == SuiAssetKey {
		return nil
	}
	return fmt.Errorf("invalid sui asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid sui address %s", address)
	}
	if !strings.HasPrefix(address, "0x") {
		return fmt.Errorf("invalid sui address prefix %s", address)
	}
	a, err := hex.DecodeString(strings.Replace(address, "0x", "", 1))
	if err != nil {
		return fmt.Errorf("invalid sui address %s %s", address, err.Error())
	}
	if len(a) != 32 {
		return fmt.Errorf("invalid sui address %s", address)
	}
	addr := fmt.Sprintf("0x%s", hex.EncodeToString(a))
	if addr != address {
		return fmt.Errorf("invalid sui address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	h, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid sui transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 33 {
		return fmt.Errorf("invalid sui transaction hash %s", hash)
	}
	if base64.StdEncoding.EncodeToString(h) != hash {
		return fmt.Errorf("invalid sui transaction hash encode %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case SuiAssetKey:
		return SuiChainId
	default:
		panic(assetKey)
	}
}
