package binance

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/ethereum"
	"github.com/btcsuite/btcd/btcutil/bech32"
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
	addr, err := AccAddressFromBech32(address)
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

	return ethereum.BuildChainAssetId(BinanceChainBase, assetKey)
}

type AccAddress []byte

// AccAddressFromBech32 to create an AccAddress from a bech32 string
func AccAddressFromBech32(address string) (addr AccAddress, err error) {
	bz, err := GetFromBech32(address, "bnb")
	if err != nil {
		return nil, err
	}
	return AccAddress(bz), nil
}

// GetFromBech32 to decode a bytestring from a bech32-encoded string
func GetFromBech32(bech32str, prefix string) ([]byte, error) {
	if len(bech32str) == 0 {
		return nil, errors.New("decoding bech32 address failed: must provide an address")
	}
	hrp, bz, err := DecodeAndConvert(bech32str)
	if err != nil {
		return nil, err
	}

	if hrp != prefix {
		return nil, fmt.Errorf("invalid bech32 prefix. Expected %s, Got %s", prefix, hrp)
	}

	return bz, nil
}

// String representation
func (bz AccAddress) String() string {
	bech32Addr, err := ConvertAndEncode("bnb", bz)
	if err != nil {
		panic(err)
	}
	return bech32Addr
}

func DecodeAndConvert(bech string) (string, []byte, error) {
	hrp, data, err := bech32.Decode(bech)
	if err != nil {
		return "", nil, err
	}
	converted, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", nil, err
	}
	return hrp, converted, nil
}

func ConvertAndEncode(hrp string, data []byte) (string, error) {
	converted, err := bech32.ConvertBits(data, 8, 5, true)
	if err != nil {
		return "", err
	}
	return bech32.Encode(hrp, converted)

}
