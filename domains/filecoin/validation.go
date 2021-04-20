package filecoin

import (
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
)

var (
	FilecoinChainBase string
	FilecoinChainId   crypto.Hash
	HashFunction      uint64
)

func init() {
	FilecoinChainBase = "08285081-e1d8-4be6-9edc-e203afa932da"
	FilecoinChainId = crypto.NewHash([]byte(FilecoinChainBase))

	address.CurrentNetwork = address.Mainnet
	HashFunction = uint64(mh.BLAKE2B_MIN + 31)
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

	if string(addr[0]) != address.MainnetPrefix {
		return fmt.Errorf("invalid filecoin address %s", addr)
	}
	a, err := address.NewFromString(addr)
	if err != nil {
		return fmt.Errorf("invalid filecoin address %s %s", addr, err)
	}
	if a.Protocol() != address.SECP256K1 && a.Protocol() != address.BLS {
		return fmt.Errorf("invalid filecoin address %s", addr)
	}
	if a.String() != addr {
		return fmt.Errorf("invalid filecoin address %s %s", addr, a.String())
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	id, err := cid.Parse(hash)
	if err != nil {
		return fmt.Errorf("invalid filecoin transaction hash %s %s", hash, err)
	}
	if id.Prefix().MhType != HashFunction {
		return fmt.Errorf("invalid filecoin transaction hash %s 0", hash)
	}
	dmh, err := mh.Decode(id.Hash())
	if err != nil {
		return fmt.Errorf("invalid filecoin transaction hash %s %s 1", hash, err)
	}
	if dmh.Code != HashFunction {
		return fmt.Errorf("invalid filecoin transaction hash %s 2", hash)
	}
	if id.String() != hash {
		return fmt.Errorf("invalid filecoin transaction hash %s %s 3", hash, id.String())
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
