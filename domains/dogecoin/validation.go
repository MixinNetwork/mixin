package dogecoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/litecoin"
)

var (
	DogecoinChainBase string
	DogecoinChainId   crypto.Hash
)

func init() {
	DogecoinChainBase = "6770a1e5-6086-44d5-b60f-545f9d9e8ffd"
	DogecoinChainId = crypto.NewHash([]byte(DogecoinChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == DogecoinChainBase {
		return nil
	}
	return fmt.Errorf("invalid dogecoin asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid dogecoin address %s", address)
	}
	dogeAddress, err := litecoin.DecodeAddress(address, &mainNetParams)
	if err != nil {
		return err
	}
	if dogeAddress.String() != address {
		return fmt.Errorf("invalid dogecoin address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid dogecoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid dogecoin transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid dogecoin transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid dogecoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case DogecoinChainBase:
		return DogecoinChainId
	default:
		panic(assetKey)
	}
}

var mainNetParams = litecoin.Params{
	PubKeyHashAddrID: 0x1e,
	ScriptHashAddrID: 0x16,
}
