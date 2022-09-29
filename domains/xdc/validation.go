package xdc

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/ethereum"
)

var (
	XDCChainBase string
	XDCChainId   crypto.Hash
)

func init() {
	XDCChainBase = "b12bb04a-1cea-401c-a086-0be61f544889"
	XDCChainId = crypto.NewHash([]byte(XDCChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if len(assetKey) != 43 {
		return fmt.Errorf("invalid xdc network asset key %s", assetKey)
	}
	if !strings.HasPrefix(assetKey, "xdc") {
		return fmt.Errorf("invalid xdc network asset key %s", assetKey)
	}
	if assetKey != strings.ToLower(assetKey) {
		return fmt.Errorf("invalid xdc network asset key %s", assetKey)
	}
	k, err := hex.DecodeString(assetKey[3:])
	if err != nil {
		return fmt.Errorf("invalid xdc network asset key %s", assetKey)
	}
	if len(k) != 20 {
		return fmt.Errorf("invalid xdc network asset key %s", assetKey)
	}
	return nil
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid xdc network address %s", address)
	}
	if len(address) != 43 {
		return fmt.Errorf("invalid xdc network address %s", address)
	}
	if !strings.HasPrefix(address, "xdc") {
		return fmt.Errorf("invalid xdc network address %s", address)
	}
	form, err := formatAddress(address)
	if err != nil {
		return fmt.Errorf("invalid xdc network address %s", address)
	}
	if form != address {
		return fmt.Errorf("invalid xdc network address %s", address)
	}
	a, err := hex.DecodeString(address[3:])
	if err != nil {
		return fmt.Errorf("invalid xdc network address %s %s", address, err.Error())
	}
	if len(a) != 20 {
		return fmt.Errorf("invalid xdc network address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 66 {
		return fmt.Errorf("invalid xdc network transaction hash %s", hash)
	}
	if !strings.HasPrefix(hash, "0x") {
		return fmt.Errorf("invalid xdc network transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid xdc network transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash[2:])
	if err != nil {
		return fmt.Errorf("invalid xdc network transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid xdc network transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	err := VerifyAssetKey(assetKey)
	if err != nil {
		panic(assetKey)
	}

	if assetKey == "xdc0000000000000000000000000000000000000000" {
		return XDCChainId
	}

	return ethereum.BuildChainAssetId(XDCChainBase, assetKey)
}

func formatAddress(to string) (string, error) {
	var bytesto [20]byte
	_bytesto, err := hex.DecodeString(to[3:])
	if err != nil {
		return "", err
	}
	copy(bytesto[:], _bytesto)
	address := ethereum.Address(bytesto)
	return "xdc" + address.Hex()[2:], nil
}
