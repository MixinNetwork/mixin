package siacoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"gitlab.com/NebulousLabs/Sia/types"
)

var (
	SiacoinChainBase string
	SiacoinChainId   crypto.Hash
)

func init() {
	SiacoinChainBase = "990c4c29-57e9-48f6-9819-7d986ea44985"
	SiacoinChainId = crypto.NewHash([]byte(SiacoinChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == SiacoinChainBase {
		return nil
	}
	return fmt.Errorf("invalid siacoin asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid siacoin address %s", address)
	}
	var uh types.UnlockHash
	err := uh.LoadString(address)
	if err != nil {
		return err
	}
	if uh.String() != address {
		return fmt.Errorf("invalid siacoin address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid siacoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid siacoin transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid siacoin transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid siacoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case SiacoinChainBase:
		return SiacoinChainId
	default:
		panic(assetKey)
	}
}
