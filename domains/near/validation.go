package near

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcutil/base58"
)

var (
	NearChainBase string
	NearChainId   crypto.Hash
)

func init() {
	NearChainBase = "d6ac94f7-c932-4e11-97dd-617867f0669e"
	NearChainId = crypto.NewHash([]byte(NearChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == NearChainBase {
		return nil
	}
	return fmt.Errorf("invalid near asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid near address %s", address)
	}
	addr, err := hex.DecodeString(address)
	if err != nil {
		return fmt.Errorf("invalid near address %s", address)
	}
	if len(addr) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid near address length %s", address)
	}
	if hex.EncodeToString(addr) != address {
		return fmt.Errorf("invalid near address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if strings.TrimSpace(hash) != hash {
		return fmt.Errorf("invalid near transaction hash %s", hash)
	}
	h := base58.Decode(hash)
	if len(h) != 32 {
		return fmt.Errorf("invalid near transaction hash base58 %s", hash)
	}
	hs := base58.Encode(h)
	if hs != hash {
		return fmt.Errorf("invalid near transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case NearChainBase:
		return NearChainId
	default:
		panic(assetKey)
	}
}
