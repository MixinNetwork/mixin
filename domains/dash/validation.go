package dash

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/litecoin"
)

var (
	DashChainBase string
	DashChainId   crypto.Hash
)

func init() {
	DashChainBase = "6472e7e3-75fd-48b6-b1dc-28d294ee1476"
	DashChainId = crypto.NewHash([]byte(DashChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == DashChainBase {
		return nil
	}
	return fmt.Errorf("invalid dash asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid dash address %s", address)
	}
	dashAddress, err := litecoin.DecodeAddress(address, &mainNetParams)
	if err != nil {
		return err
	}
	if dashAddress.String() != address {
		return fmt.Errorf("invalid dash address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid dash transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid dash transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid dash transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid dash transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case DashChainBase:
		return DashChainId
	default:
		panic(assetKey)
	}
}

var mainNetParams = litecoin.Params{
	PubKeyHashAddrID: 0x4c,
	ScriptHashAddrID: 0x10,
}
