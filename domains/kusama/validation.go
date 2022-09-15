package kusama

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/util/base58"
	"golang.org/x/crypto/blake2b"
)

const (
	addressPrefix = 2
)

var (
	KusamaChainBase string
	KusamaChainId   crypto.Hash
)

func init() {
	KusamaChainBase = "9d29e4f6-d67c-4c4b-9525-604b04afbe9f"
	KusamaChainId = crypto.NewHash([]byte(KusamaChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == KusamaChainBase {
		return nil
	}
	return fmt.Errorf("invalid kusama asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid kusama address %s", address)
	}
	b := base58.Decode(address)
	if len(b) < 8 {
		return fmt.Errorf("invalid kusama address %s", address)
	}
	if b[0] != byte(addressPrefix) {
		return fmt.Errorf("invalid kusama address %s", address)
	}
	addr := base58.Encode(b)
	if addr != address {
		return fmt.Errorf("invalid kusama address %s", address)
	}
	public := b[1 : len(b)-2]
	addr, err := PublicKeyToAddress(public)
	if err != nil {
		return fmt.Errorf("invalid kusama address %s", address)
	}
	if addr != address {
		return fmt.Errorf("invalid kusama address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 66 {
		return fmt.Errorf("invalid kusama transaction hash %s", hash)
	}
	if !strings.HasPrefix(hash, "0x") {
		return fmt.Errorf("invalid kusama transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid kusama transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash[2:])
	if err != nil {
		return fmt.Errorf("invalid kusama transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid kusama transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case KusamaChainBase:
		return KusamaChainId
	default:
		panic(assetKey)
	}
}

var ss58Prefix = []byte("SS58PRE")

func PublicKeyToAddress(pub []byte) (string, error) {
	enc := append([]byte{addressPrefix}, pub...)
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
