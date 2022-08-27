package terra

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/gofrs/uuid"
)

var (
	TerraAssetKey  string
	TerraChainBase string
	TerraChainId   crypto.Hash
)

func init() {
	TerraAssetKey = "uluna"
	TerraChainBase = "eb5bb26d-bfda-4e63-bf1d-a462b78343b7"
	TerraChainId = crypto.NewHash([]byte(TerraChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == TerraAssetKey {
		return nil
	}
	err := VerifyAddress(assetKey)
	if err == nil {
		return nil
	}
	err = validateDenom(assetKey)
	if err == nil {
		return nil
	}
	return fmt.Errorf("invalid terra asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid terra address %s", address)
	}

	bech32Prefix := "terra"
	hrp, bz, err := decodeAndConvert(address)
	if err != nil {
		return fmt.Errorf("invalid terra address %s %s", address, err.Error())
	}
	if hrp != bech32Prefix {
		return fmt.Errorf("invalid terra address %s", address)
	}
	if len(bz) != 20 {
		return fmt.Errorf("invalid terra address %s", address)
	}
	addr, err := convertAndEncode(bech32Prefix, bz)
	if err != nil {
		return fmt.Errorf("invalid terra address %s %s", address, err.Error())
	}
	if addr != address {
		return fmt.Errorf("invalid terra address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid terra transaction hash %s %s", hash, err.Error())
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid bitcoin transaction hash %s", hash)
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid terra transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	if assetKey == TerraAssetKey {
		return TerraChainId
	}
	if VerifyAssetKey(assetKey) != nil {
		panic(assetKey)
	}

	h := md5.New()
	io.WriteString(h, TerraChainBase)
	io.WriteString(h, assetKey)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	id := uuid.FromBytesOrNil(sum).String()
	return crypto.NewHash([]byte(id))
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

func validateDenom(denom string) error {
	reDnmString := `[a-zA-Z][a-zA-Z0-9/-]{2,127}`
	reDnm := regexp.MustCompile(fmt.Sprintf(`^%s$`, reDnmString))
	if !reDnm.MatchString(denom) {
		return fmt.Errorf("invalid denom: %s", denom)
	}
	return nil
}
