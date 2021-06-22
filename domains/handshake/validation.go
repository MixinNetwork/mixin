package handshake

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/litecoin"
)

var (
	HandshakenChainBase string
	HandshakenChainId   crypto.Hash
	Bech32HRPSegwit     = "hs"
)

func init() {
	HandshakenChainBase = "13036886-6b83-4ced-8d44-9f69151587bf"
	HandshakenChainId = crypto.NewHash([]byte(HandshakenChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == HandshakenChainBase {
		return nil
	}
	return fmt.Errorf("invalid handshake asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid handshake address %s", address)
	}
	if !strings.HasPrefix(address, Bech32HRPSegwit+"1") {
		return fmt.Errorf("invalid handshake address %s", address)
	}
	addr, err := litecoin.DecodeAddress(address, &litecoin.Params{Bech32HRPSegwit: Bech32HRPSegwit})
	if err != nil {
		return fmt.Errorf("invalid handshake address %s", address)
	}
	if addr.String() != address {
		return fmt.Errorf("invalid handshake address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid handshake transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid handshake transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid handshake transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid handshake transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case HandshakenChainBase:
		return HandshakenChainId
	default:
		panic(assetKey)
	}
}
