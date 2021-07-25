package horizen

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/zcash"
)

var (
	HorizenChainBase string
	HorizenChainId   crypto.Hash
)

func init() {
	HorizenChainBase = "a2c5d22b-62a2-4c13-b3f0-013290dbac60"
	HorizenChainId = crypto.NewHash([]byte(HorizenChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == HorizenChainBase {
		return nil
	}
	return fmt.Errorf("invalid horizen asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid horizen address %s", address)
	}
	horizenAddress, err := zcash.DecodeAddress(address, &mainNetParams)
	if err != nil {
		return err
	}
	if horizenAddress.EncodeAddress() != address {
		return fmt.Errorf("invalid horizen address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid horizen transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid horizen transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid horizen transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid horizen transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case HorizenChainBase:
		return HorizenChainId
	default:
		panic(assetKey)
	}
}

var mainNetParams = zcash.Params{
	PubKeyBase58Prefixes: [2]byte{0x20, 0x89},
	ScriptBase58Prefixes: [2]byte{0x20, 0x96},
}
