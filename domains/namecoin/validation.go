package namecoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/litecoin"
)

var (
	NamecoinChainBase string
	NamecoinChainId   crypto.Hash
)

func init() {
	NamecoinChainBase = "f8b77dc0-46fd-4ea1-9821-587342475869"
	NamecoinChainId = crypto.NewHash([]byte(NamecoinChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == NamecoinChainBase {
		return nil
	}
	return fmt.Errorf("invalid namecoin asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid namecoin address %s", address)
	}
	nmcAddress, err := litecoin.DecodeAddress(address, &mainNetParams)
	if err != nil {
		return err
	}
	if nmcAddress.String() != address {
		return fmt.Errorf("invalid namecoin address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid namecoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid namecoin transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid namecoin transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid namecoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case NamecoinChainBase:
		return NamecoinChainId
	default:
		panic(assetKey)
	}
}

var mainNetParams = litecoin.Params{
	PubKeyHashAddrID: 0x34,
	ScriptHashAddrID: 0x0d,
}
