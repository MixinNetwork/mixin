package mobilecoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/ethereum"
	account "github.com/jadeydi/mobilecoin-account"
)

var (
	MobileCoinChainBase string
	MobileCoinChainId   crypto.Hash
)

func init() {
	MobileCoinChainBase = "eea900a8-b327-488c-8d8d-1428702fe240"
	MobileCoinChainId = crypto.NewHash([]byte(MobileCoinChainBase))
}

func VerifyAssetKey(assetKey string) error {
	switch assetKey {
	case MobileCoinChainBase:
		return nil
	case "0", "1":
		return nil
	default:
		return fmt.Errorf("invalid mobilecoin asset key %s", assetKey)
	}
}

func VerifyAddress(address string) error {
	am, err := account.DecodeB58Code(address)
	if err != nil {
		return err
	}
	if len(am.ViewPublicKey) != 64 || len(am.SpendPublicKey) != 64 {
		return fmt.Errorf("Invalid mobilecoin address: %s", address)
	}
	mobAddress, err := am.B58Code()
	if err != nil {
		return err
	}
	if mobAddress != address {
		return fmt.Errorf("Invalid mobilecoin address %s %s", address, mobAddress)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid mobilecoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid mobilecoin transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid mobilecoin transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid mobilecoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case MobileCoinChainBase:
		return MobileCoinChainId
	case "0":
		return MobileCoinChainId
	case "1":
		return ethereum.BuildChainAssetId(MobileCoinChainBase, assetKey)
	default:
		panic(assetKey)
	}
}
