package bch

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/bch/bchutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

var (
	BitcoinCashChainBase string
	BitcoinCashChainId   crypto.Hash
)

func init() {
	BitcoinCashChainBase = "fd11b6e3-0b87-41f1-a41f-f0e9b49e5bf0"
	BitcoinCashChainId = crypto.NewHash([]byte(BitcoinCashChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == BitcoinCashChainBase {
		return nil
	}
	return fmt.Errorf("invalid bitcoin cash asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid bitcoin cash address %s", address)
	}
	if strings.HasPrefix(address, "bitcoincash:") {
		bchAddress, err := bchutil.DecodeAddress(address, &chaincfg.MainNetParams)
		if err != nil {
			return fmt.Errorf("invalid bitcoin cash address %s %s", address, err)
		}
		addr := "bitcoincash:" + bchAddress.EncodeAddress()
		if addr != address {
			return fmt.Errorf("invalid bitcoin cash address %s", address)
		}
		return nil
	}

	btcAddress, err := btcutil.DecodeAddress(address, &chaincfg.MainNetParams)
	if err != nil {
		return fmt.Errorf("invalid bitcoin cash address %s %s", address, err)
	}
	if btcAddress.String() != address {
		return fmt.Errorf("invalid bitcoin cash address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid bitcoin cash transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid bitcoin cash transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid bitcoin cash transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid bitcoin cash transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case BitcoinCashChainBase:
		return BitcoinCashChainId
	default:
		panic(assetKey)
	}
}
