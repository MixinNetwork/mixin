package starcoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	StarcoinAssetKey  string
	StarcoinChainBase string
	StarcoinChainId   crypto.Hash
)

func init() {
	StarcoinAssetKey = "0x00000000000000000000000000000001::STC::STC"
	StarcoinChainBase = "c99a3779-93df-404d-945d-eddc440aa0b2"
	StarcoinChainId = crypto.NewHash([]byte(StarcoinChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == StarcoinAssetKey {
		return nil
	}
	return fmt.Errorf("invalid starcoin asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid starcoin address %s", address)
	}
	if !strings.HasPrefix(address, "0x") {
		return fmt.Errorf("invalid starcoin address prefix %s", address)
	}
	a, err := hex.DecodeString(strings.Replace(address, "0x", "", 1))
	if err != nil {
		return fmt.Errorf("invalid starcoin address %s %s", address, err.Error())
	}
	if len(a) != 16 {
		return fmt.Errorf("invalid starcoin address %s", address)
	}
	if !strings.EqualFold(hex.EncodeToString(a), address[2:]) {
		return fmt.Errorf("invalid starcoin address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if !strings.HasPrefix(hash, "0x") {
		return fmt.Errorf("invalid starcoin transaction hash prefix %s", hash)
	}
	h, err := hex.DecodeString(strings.Replace(hash, "0x", "", 1))
	if err != nil {
		return fmt.Errorf("invalid starcoin transaction hash %s %s", hash, err.Error())
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid starcoin transaction hash %s", hash)
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid starcoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case StarcoinAssetKey:
		return StarcoinChainId
	default:
		panic(assetKey)
	}
}
