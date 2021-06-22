package litecoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	LitecoinChainBase string
	LitecoinChainId   crypto.Hash
)

func init() {
	LitecoinChainBase = "76c802a2-7c88-447f-a93e-c29c9e5dd9c8"
	LitecoinChainId = crypto.NewHash([]byte(LitecoinChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == LitecoinChainBase {
		return nil
	}
	return fmt.Errorf("invalid litecoin asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid litecoin address %s", address)
	}
	ltcAddress, err := DecodeAddress(address, &ltcParams)
	if err != nil {
		ltcAddress, err = DecodeAddress(address, &legacyParams)
		if err != nil {
			return fmt.Errorf("invalid litecoin address %s %s", address, err.Error())
		}
	}
	if ltcAddress.String() != address {
		return fmt.Errorf("invalid litecoin address %s %s", ltcAddress.String(), address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid litecoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid litecoin transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid litecoin transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid litecoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case LitecoinChainBase:
		return LitecoinChainId
	default:
		panic(assetKey)
	}
}

var (
	ltcParams = Params{
		Bech32HRPSegwit:  "ltc",
		PubKeyHashAddrID: 0x30,
		ScriptHashAddrID: 0x32,
	}
	legacyParams = Params{
		Bech32HRPSegwit:  "ltc",
		PubKeyHashAddrID: 0x30,
		ScriptHashAddrID: 0x05,
	}
)
