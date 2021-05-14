package arweave

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	ArweaveChainBase string
	ArweaveChainId   crypto.Hash
)

func init() {
	ArweaveChainBase = "882eb041-64ea-465f-a4da-817bd3020f52"
	ArweaveChainId = crypto.NewHash([]byte(ArweaveChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == ArweaveChainBase {
		return nil
	}
	return fmt.Errorf("invalid arweave asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid arweave address %s", address)
	}

	addr, err := base64.RawURLEncoding.DecodeString(address)
	if err != nil {
		return fmt.Errorf("invalid arweave address %s", address)
	}
	if len(addr) != 32 {
		return fmt.Errorf("invalid arweave address length %d not equal 32", len(addr))
	}
	arAddress := base64.RawURLEncoding.EncodeToString(addr)
	if arAddress != address {
		return fmt.Errorf("invalid arweave address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if strings.TrimSpace(hash) != hash {
		return fmt.Errorf("invalid arweave transaction hash %s", hash)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid arweave transaction hash %s %s", hash, err.Error())
	}
	if len(decoded) != 32 {
		return fmt.Errorf("invalid arweave transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case ArweaveChainBase:
		return ArweaveChainId
	default:
		panic(assetKey)
	}
}
