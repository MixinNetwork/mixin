package cosmos

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcd/btcutil/bech32"
)

var (
	CosmosAssetKey  string
	CosmosChainBase string
	CosmosChainId   crypto.Hash
)

func init() {
	CosmosAssetKey = "uatom"
	CosmosChainBase = "7397e9f1-4e42-4dc8-8a3b-171daaadd436"
	CosmosChainId = crypto.NewHash([]byte(CosmosChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == CosmosAssetKey {
		return nil
	}
	return fmt.Errorf("invalid cosmos asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid cosmos address %s", address)
	}

	bech32Prefix := "cosmos"
	hrp, bz, err := decodeAndConvert(address)
	if err != nil {
		return fmt.Errorf("invalid cosmos address %s %s", address, err.Error())
	}
	if hrp != bech32Prefix {
		return fmt.Errorf("invalid cosmos address %s", address)
	}
	if len(bz) != 20 {
		return fmt.Errorf("invalid cosmos address %s", address)
	}
	addr, err := convertAndEncode(bech32Prefix, bz)
	if err != nil {
		return fmt.Errorf("invalid cosmos address %s %s", address, err.Error())
	}
	if addr != address {
		return fmt.Errorf("invalid cosmos address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid cosmos transaction hash %s %s", hash, err.Error())
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid cosmos transaction hash %s", hash)
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid cosmos transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case CosmosAssetKey:
		return CosmosChainId
	default:
		panic(assetKey)
	}
}

func convertAndEncode(hrp string, data []byte) (string, error) {
	converted, err := bech32.ConvertBits(data, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("encoding bech32 failed: %w", err)
	}

	return bech32.Encode(hrp, converted)
}

func decodeAndConvert(bech string) (string, []byte, error) {
	if len(bech) > 1023 {
		return "", nil, fmt.Errorf("invalid bech32 string length %d",
			len(bech))
	}
	hrp, data, err := bech32.DecodeNoLimit(bech)
	if err != nil {
		return "", nil, fmt.Errorf("decoding bech32 failed: %w", err)
	}

	converted, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", nil, fmt.Errorf("decoding bech32 failed: %w", err)
	}
	return hrp, converted, nil
}
