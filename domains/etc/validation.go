package etc

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/ethereum"
)

var (
	EthereumClassicChainBase string
	EthereumClassicChainId   crypto.Hash
)

func init() {
	EthereumClassicChainBase = "2204c1ee-0ea2-4add-bb9a-b3719cfff93a"
	EthereumClassicChainId = crypto.NewHash([]byte(EthereumClassicChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == "0x0000000000000000000000000000000000000000" {
		return nil
	}
	return fmt.Errorf("invalid ethereum classic asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid ethereum classic address %s", address)
	}
	if len(address) != 42 {
		return fmt.Errorf("invalid ethereum classic address %s", address)
	}
	if !strings.HasPrefix(address, "0x") {
		return fmt.Errorf("invalid ethereum classic address %s", address)
	}
	form, err := formatAddress(address)
	if err != nil {
		return fmt.Errorf("invalid ethereum classic address %s", address)
	}
	if form != address {
		return fmt.Errorf("invalid ethereum classic address %s", address)
	}
	a, err := hex.DecodeString(address[2:])
	if err != nil {
		return fmt.Errorf("invalid ethereum classic address %s %s", address, err.Error())
	}
	if len(a) != 20 {
		return fmt.Errorf("invalid ethereum classic address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 66 {
		return fmt.Errorf("invalid ethereum classic transaction hash %s", hash)
	}
	if !strings.HasPrefix(hash, "0x") {
		return fmt.Errorf("invalid ethereum classic transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid ethereum classic transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash[2:])
	if err != nil {
		return fmt.Errorf("invalid ethereum classic transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid ethereum classic transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	err := VerifyAssetKey(assetKey)
	if err != nil {
		panic(assetKey)
	}

	if assetKey == "0x0000000000000000000000000000000000000000" {
		return EthereumClassicChainId
	}
	panic(assetKey)
}

func formatAddress(to string) (string, error) {
	var bytesto [20]byte
	_bytesto, err := hex.DecodeString(to[2:])
	if err != nil {
		return "", err
	}
	copy(bytesto[:], _bytesto)
	address := ethereum.Address(bytesto)
	return address.Hex(), nil
}
