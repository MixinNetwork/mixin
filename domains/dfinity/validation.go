package dfinity

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	DfinityChainBase = "d5db6f39-fe50-4633-8edc-36e2f3e117e4"
)

var (
	DfinityChainId crypto.Hash
)

func init() {
	DfinityChainId = crypto.NewHash([]byte(DfinityChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == DfinityChainBase {
		return nil
	}
	return fmt.Errorf("invalid internet computer asset key %s", assetKey)
}

func VerifyAddress(addr string) error {
	if strings.TrimSpace(addr) != addr {
		return fmt.Errorf("invalid internet computer address %s", addr)
	}

	buf, err := hex.DecodeString(addr)
	if err != nil {
		return err
	}
	if len(buf) != 32 {
		return fmt.Errorf("invalid internet computer address %s", addr)
	}
	result := binary.BigEndian.Uint32(buf[:4])
	if result != crc32.ChecksumIEEE(buf[4:]) {
		return fmt.Errorf("invalid internet computer address %s", addr)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid internet computer transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid internet computer transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid internet computer transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid internet computer transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case DfinityChainBase:
		return DfinityChainId
	default:
		panic(assetKey)
	}
}
