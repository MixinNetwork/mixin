package decred

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/decred/dcrd/dcrutil"
)

var (
	DecredChainBase string
	DecredChainId   crypto.Hash
)

func init() {
	DecredChainBase = "8f5caf2a-283d-4c85-832a-91e83bbf290b"
	DecredChainId = crypto.NewHash([]byte(DecredChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == DecredChainBase {
		return nil
	}
	return fmt.Errorf("invalid decred asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid decred address %s", address)
	}
	dcrAddress, err := dcrutil.DecodeAddress(address)
	if err != nil {
		return fmt.Errorf("invalid decred address %s %s", address, err)
	}
	if dcrAddress.String() != address {
		return fmt.Errorf("invalid decred address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid decred transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid decred transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid decred transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid decred transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case DecredChainBase:
		return DecredChainId
	default:
		panic(assetKey)
	}
}
