package polkadot

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/blake2b"
)

var (
	PolkadotChainBase string
	PolkadotChainId   crypto.Hash
)

func init() {
	PolkadotChainBase = "54c61a72-b982-4034-a556-0d99e3c21e39"
	PolkadotChainId = crypto.NewHash([]byte(PolkadotChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == PolkadotChainBase {
		return nil
	}
	return fmt.Errorf("invalid polkadot asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid polkadot address %s", address)
	}
	b := base58.Decode(address)
	if len(b) < 8 {
		return fmt.Errorf("invalid polkadot address %s", address)
	}
	if b[0] != byte(0) {
		return fmt.Errorf("invalid polkadot address %s", address)
	}
	addr := base58.Encode(b)
	if addr != address {
		return fmt.Errorf("invalid polkadot address %s", address)
	}
	public := b[1 : len(b)-2]
	addr, err := PublicKeyToAddress(public)
	if err != nil {
		return fmt.Errorf("invalid polkadot address %s", address)
	}
	if addr != address {
		return fmt.Errorf("invalid polkadot address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 66 {
		return fmt.Errorf("invalid polkadot transaction hash %s", hash)
	}
	if !strings.HasPrefix(hash, "0x") {
		return fmt.Errorf("invalid polkadot transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid polkadot transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash[2:])
	if err != nil {
		return fmt.Errorf("invalid polkadot transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid polkadot transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case PolkadotChainBase:
		return PolkadotChainId
	default:
		panic(assetKey)
	}
}

var ss58Prefix = []byte("SS58PRE")

func PublicKeyToAddress(pub []byte) (string, error) {
	enc := append([]byte{0}, pub...)
	hasher, err := blake2b.New(64, nil)
	if err != nil {
		return "", err
	}
	_, err = hasher.Write(append(ss58Prefix, enc...))
	if err != nil {
		return "", err
	}
	checksum := hasher.Sum(nil)
	return base58.Encode(append(enc, checksum[:2]...)), nil
}
