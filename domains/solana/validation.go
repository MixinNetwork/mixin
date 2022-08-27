package solana

import (
	"crypto/md5"
	"fmt"
	"io"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcd/btcutil/base58"
	"github.com/gofrs/uuid"
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
	pub := base58.Decode(assetKey)
	if len(pub) != 32 {
		return fmt.Errorf("invalid solana assetKey length %s", assetKey)
	}
	var k crypto.Key
	copy(k[:], pub)
	if !k.CheckKey() {
		return fmt.Errorf("invalid solana assetKey public key %s", assetKey)
	}
	addr := base58.Encode(pub)
	if addr != assetKey {
		return fmt.Errorf("invalid solana assetKey %s", assetKey)
	}
	return nil
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
	if assetKey == "11111111111111111111111111111111" {
		return SolanaChainId
	}
	err := VerifyAssetKey(assetKey)
	if err != nil {
		panic(assetKey)
	}

	h := md5.New()
	io.WriteString(h, SolanaChainBase)
	io.WriteString(h, assetKey)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	id := uuid.FromBytesOrNil(sum).String()
	return crypto.NewHash([]byte(id))
}
