package ravencoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

var (
	RavencoinChainBase string
	RavencoinChainId   crypto.Hash
)

func init() {
	RavencoinChainBase = "6877d485-6b64-4225-8d7e-7333393cb243"
	RavencoinChainId = crypto.NewHash([]byte(RavencoinChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == RavencoinChainBase {
		return nil
	}
	return fmt.Errorf("invalid ravencoin asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid ravencoin address %s", address)
	}
	rvnAddress, err := btcutil.DecodeAddress(address, &mainNetParams)
	if err != nil {
		return err
	}
	if rvnAddress.String() != address {
		return fmt.Errorf("invalid ravencoin address %s", address)
	}
	if !rvnAddress.IsForNet(&mainNetParams) {
		return fmt.Errorf("invalid ravencoin address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid ravencoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid ravencoin transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid ravencoin transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid ravencoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case RavencoinChainBase:
		return RavencoinChainId
	default:
		panic(assetKey)
	}
}

var mainNetParams = chaincfg.Params{
	Net:              0x4e564152,
	Name:             "main",
	DefaultPort:      "8767",
	PubKeyHashAddrID: 0x3c,
	ScriptHashAddrID: 0x7a,
	PrivateKeyID:     0x80,

	HDPrivateKeyID: [4]byte{0x04, 0x88, 0xad, 0xe4},
	HDPublicKeyID:  [4]byte{0x04, 0x88, 0xb2, 0x1e},
}
