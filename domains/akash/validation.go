package akash

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/util/bech32"
)

var (
	AkashAssetKey  string
	AkashChainBase string
	AkashChainId   crypto.Hash
)

func init() {
	AkashAssetKey = "uakt"
	AkashChainBase = "9c612618-ca59-4583-af34-be9482f5002d"
	AkashChainId = crypto.NewHash([]byte(AkashChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == AkashAssetKey {
		return nil
	}
	return fmt.Errorf("invalid akash asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid akash address %s", address)
	}

	bech32Prefix := "akash"
	hrp, bz, err := decodeAndConvert(address)
	if err != nil {
		return fmt.Errorf("invalid akash address %s %s", address, err.Error())
	}
	if hrp != bech32Prefix {
		return fmt.Errorf("invalid akash address %s", address)
	}
	if len(bz) != 20 {
		return fmt.Errorf("invalid akash address %s", address)
	}
	addr, err := convertAndEncode(bech32Prefix, bz)
	if err != nil {
		return fmt.Errorf("invalid akash address %s %s", address, err.Error())
	}
	if addr != address {
		return fmt.Errorf("invalid akash address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid akash transaction hash %s %s", hash, err.Error())
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid bitcoin transaction hash %s", hash)
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid akash transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case AkashAssetKey:
		return AkashChainId
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
