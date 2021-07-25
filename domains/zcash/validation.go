package zcash

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	ZcashChainBase string
	ZcashChainId   crypto.Hash
)

func init() {
	ZcashChainBase = "c996abc9-d94e-4494-b1cf-2a3fd3ac5714"
	ZcashChainId = crypto.NewHash([]byte(ZcashChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == ZcashChainBase {
		return nil
	}
	return fmt.Errorf("invalid zcash asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid zcash address %s", address)
	}
	zcashAddress, err := DecodeAddress(address, &mainNetParams)
	if err != nil {
		return err
	}
	if zcashAddress.EncodeAddress() != address {
		return fmt.Errorf("invalid zcash address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid zcash transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid zcash transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid zcash transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid zcash transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case ZcashChainBase:
		return ZcashChainId
	default:
		panic(assetKey)
	}
}

var mainNetParams = Params{
	PubKeyBase58Prefixes: [2]byte{0x1C, 0xB8},
	ScriptBase58Prefixes: [2]byte{0x1C, 0xBD},
}
