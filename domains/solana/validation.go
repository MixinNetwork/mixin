package solana

import (
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcd/btcutil/base58"
)

var (
	SolanaChainBase string
	SolanaChainId   crypto.Hash
)

func init() {
	SolanaChainBase = "64692c23-8971-4cf4-84a7-4dd1271dd887"
	SolanaChainId = crypto.NewHash([]byte(SolanaChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == "11111111111111111111111111111111" {
		return nil
	}
	return fmt.Errorf("invalid solana asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid solana address %s", address)
	}
	pub := base58.Decode(address)
	if len(pub) != 32 {
		return fmt.Errorf("invalid solana address %s", address)
	}
	var k crypto.Key
	copy(k[:], pub)
	if !k.CheckKey() {
		return fmt.Errorf("invalid solana address %s", address)
	}
	addr := base58.Encode(pub)
	if addr != address {
		return fmt.Errorf("invalid solana address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if strings.TrimSpace(hash) != hash {
		return fmt.Errorf("invalid solana transaction hash %s", hash)
	}
	h := base58.Decode(hash)
	if len(h) != 64 {
		return fmt.Errorf("invalid solana transaction hash %s", hash)
	}
	hs := base58.Encode(h)
	if hs != hash {
		return fmt.Errorf("invalid solana transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case "11111111111111111111111111111111":
		return SolanaChainId
	default:
		panic(assetKey)
	}
}
