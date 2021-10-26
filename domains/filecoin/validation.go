package filecoin

import (
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	FilecoinChainBase string
	FilecoinChainId   crypto.Hash
)

func init() {
	FilecoinChainBase = "08285081-e1d8-4be6-9edc-e203afa932da"
	FilecoinChainId = crypto.NewHash([]byte(FilecoinChainBase))

	CurrentNetwork = Mainnet
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == FilecoinChainBase {
		return nil
	}
	return fmt.Errorf("invalid filecoin asset key %s", assetKey)
}

func VerifyAddress(addr string) error {
	if strings.TrimSpace(addr) != addr {
		return fmt.Errorf("invalid filecoin address %s", addr)
	}

	if string(addr[0]) != MainnetPrefix {
		return fmt.Errorf("invalid filecoin address %s", addr)
	}
	a, err := NewFromString(addr)
	if err != nil {
		return fmt.Errorf("invalid filecoin address %s %s", addr, err)
	}
	if a.Protocol() != SECP256K1 && a.Protocol() != BLS {
		return fmt.Errorf("invalid filecoin address %s", addr)
	}
	if a.String() != addr {
		return fmt.Errorf("invalid filecoin address %s %s", addr, a.String())
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if strings.TrimSpace(hash) != hash {
		return fmt.Errorf("invalid filecoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid filecoin transaction hash %s", hash)
	}
	if len(hash) < 32 {
		return fmt.Errorf("invalid filecoin transaction hash %s", hash)
	}
	if hash[0] != 'b' {
		return fmt.Errorf("invalid filecoin transaction hash %s", hash)
	}
	bb, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(hash[1:]))
	if err != nil {
		return fmt.Errorf("invalid filecoin transaction hash %s %s", hash, err)
	}

	vers, n, err := FromUvarint(bb)
	if err != nil {
		return fmt.Errorf("invalid filecoin transaction hash %s %s", hash, err)
	}
	if vers != 1 {
		return fmt.Errorf("invalid filecoin transaction hash %s %s", hash, err)
	}
	_, cn, err := FromUvarint(bb[n:])
	if err != nil {
		return fmt.Errorf("invalid filecoin transaction hash %s %s", hash, err)
	}

	code, n, err := FromUvarint(bb[n+cn:])
	if err != nil || n <= 0 {
		return fmt.Errorf("invalid filecoin transaction hash %s %s", hash, err)
	}
	if code != 45600 {
		return fmt.Errorf("invalid filecoin transaction hash %s 2", hash)
	}
	id := "b" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bb))
	if id != hash {
		return fmt.Errorf("invalid filecoin transaction hash %s %s 3", hash, id)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case FilecoinChainBase:
		return FilecoinChainId
	default:
		panic(assetKey)
	}
}
