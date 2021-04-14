package binance

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/binance-chain/go-sdk/common/types"
	"github.com/gofrs/uuid"
)

var (
	BinanceChainBase string
	BinanceChainId   crypto.Hash
)

func init() {
	BinanceChainBase = "17f78d7c-ed96-40ff-980c-5dc62fecbc85"
	BinanceChainId = crypto.NewHash([]byte(BinanceChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if strings.TrimSpace(assetKey) != assetKey {
		return fmt.Errorf("invalid binance asset key %s", assetKey)
	}
	if strings.ToUpper(assetKey) != assetKey {
		return fmt.Errorf("invalid binance asset key %s", assetKey)
	}
	if len(assetKey) < 1 || len(assetKey) > 16 {
		return fmt.Errorf("invalid binance asset key %s", assetKey)
	}
	return nil
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid binance address %s", address)
	}
	addr, err := types.AccAddressFromBech32(address)
	if err != nil {
		return fmt.Errorf("invalid binance address %s %s", address, err)
	}
	if addr.String() != address {
		return fmt.Errorf("invalid binance address %s %s", address, addr.String())
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid binance transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid binance transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid binance transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid binance transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	err := VerifyAssetKey(assetKey)
	if err != nil {
		panic(assetKey)
	}

	if assetKey == "BNB" {
		return BinanceChainId
	}

	h := md5.New()
	io.WriteString(h, BinanceChainBase)
	io.WriteString(h, assetKey)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	id := uuid.FromBytesOrNil(sum).String()
	return crypto.NewHash([]byte(id))
}
