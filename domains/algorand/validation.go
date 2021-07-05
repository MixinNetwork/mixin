package algorand

import (
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/algorand/go-algorand-sdk/types"
)

var (
	AlgorandChainBase string
	AlgorandChainId   crypto.Hash
)

func init() {
	AlgorandChainBase = "706b6f84-3333-4e55-8e89-275e71ce9803"
	AlgorandChainId = crypto.NewHash([]byte(AlgorandChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == AlgorandChainBase {
		return nil
	}
	return fmt.Errorf("invalid algorand asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid algorand address %s", address)
	}
	addr, err := types.DecodeAddress(address)
	if err != nil {
		return fmt.Errorf("invalid algorand address %s", address)
	}
	if addr.String() != address {
		return fmt.Errorf("invalid algorand address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 52 {
		return fmt.Errorf("invalid algorand transaction hash %s", hash)
	}
	if strings.ToUpper(hash) != hash {
		return fmt.Errorf("invalid algorand transaction hash %s", hash)
	}
	h, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid algorand transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid algorand transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case AlgorandChainBase:
		return AlgorandChainId
	default:
		panic(assetKey)
	}
}
