package litecoin

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
)

var (
	LitecoinChainBase string
	LitecoinChainId   crypto.Hash
)

func init() {
	LitecoinChainBase = "76c802a2-7c88-447f-a93e-c29c9e5dd9c8"
	LitecoinChainId = crypto.NewHash([]byte(LitecoinChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == LitecoinChainBase {
		return nil
	}
	return fmt.Errorf("invalid litecoin asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid litecoin address %s", address)
	}
	ltcAddress, err := btcutil.DecodeAddress(address, &ltcMainNetParams)
	if err != nil {
		return fmt.Errorf("invalid litecoin address %s %s", address, err.Error())
	}
	if ltcAddress.String() != address {
		return fmt.Errorf("invalid litecoin address %s %s", ltcAddress.String(), address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid litecoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid litecoin transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid litecoin transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid litecoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case LitecoinChainBase:
		return LitecoinChainId
	default:
		panic(assetKey)
	}
}

var ltcMainNetParams = chaincfg.Params{
	Name:            "mainnet",
	Bech32HRPSegwit: "ltc",

	PubKeyHashAddrID:        0x30,
	ScriptHashAddrID:        0x05,
	PrivateKeyID:            0xB0,
	WitnessPubKeyHashAddrID: 0x06,
	WitnessScriptHashAddrID: 0x0A,

	HDPrivateKeyID: [4]byte{0x04, 0x88, 0xad, 0xe4},
	HDPublicKeyID:  [4]byte{0x04, 0x88, 0xb2, 0x1e},
}
