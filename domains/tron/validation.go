package tron

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcutil/base58"
	"github.com/gofrs/uuid"
)

var (
	TronChainBase string
	TronChainId   crypto.Hash
)

func init() {
	TronChainBase = "25dabac5-056a-48ff-b9f9-f67395dc407c"
	TronChainId = crypto.NewHash([]byte(TronChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == TronChainBase {
		return nil
	}
	if strings.TrimSpace(assetKey) != assetKey {
		return fmt.Errorf("invalid tron asset key %s", assetKey)
	}
	if len(assetKey) == 7 {
		if _, err := strconv.Atoi(assetKey); err != nil {
			return fmt.Errorf("invalid tron asset key %s", assetKey)
		}
		return nil
	}
	if !strings.HasPrefix(assetKey, "T") {
		return fmt.Errorf("invalid tron asset key %s", assetKey)
	}
	form, err := formatAddress(assetKey)
	if err != nil {
		return fmt.Errorf("invalid tron asset key %s", assetKey)
	}
	if form != assetKey {
		return fmt.Errorf("invalid tron asset key %s", assetKey)
	}
	return nil
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid tron address %s", address)
	}
	if !strings.HasPrefix(address, "T") {
		return fmt.Errorf("invalid tron address %s", address)
	}
	form, err := formatAddress(address)
	if err != nil {
		return fmt.Errorf("invalid tron address %s", address)
	}
	if form != address {
		return fmt.Errorf("invalid tron address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid tron transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid tron transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid tron transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid tron transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	err := VerifyAssetKey(assetKey)
	if err != nil {
		panic(assetKey)
	}

	if assetKey == TronChainBase {
		return TronChainId
	}

	h := md5.New()
	io.WriteString(h, TronChainBase)
	io.WriteString(h, assetKey)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	id := uuid.FromBytesOrNil(sum).String()
	return crypto.NewHash([]byte(id))
}

func formatAddress(to string) (string, error) {
	result, version, err := base58.CheckDecode(to)
	if err != nil {
		return "", err
	}
	if version != 0x41 {
		return "", fmt.Errorf("invalid tron address version %d", version)
	}
	if len(result) != 20 {
		return "", fmt.Errorf("invalid tron address length %d", len(result))
	}
	return base58.CheckEncode(result, version), nil
}
