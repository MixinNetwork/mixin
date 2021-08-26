package bytom

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/litecoin"
)

var (
	BytomAssetKey  string
	BytomChainBase string
	BytomChainId   crypto.Hash
)

func init() {
	BytomAssetKey = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	BytomChainBase = "71a0e8b5-a289-4845-b661-2b70ff9968aa"
	BytomChainId = crypto.NewHash([]byte(BytomChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == BytomAssetKey {
		return nil
	}
	return fmt.Errorf("invalid bytom asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid bytom address %s", address)
	}
	bytomAddress, err := litecoin.DecodeAddress(address, &mainNetParams)
	if err != nil {
		return err
	}
	fmt.Println(bytomAddress.String())
	if bytomAddress.String() != address {
		return fmt.Errorf("invalid bytom address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid bytom transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid bytom transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid bytom transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid bytom transaction hash %s", hash)
	}
	if hash == BytomAssetKey {
		return fmt.Errorf("invalid bytom transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case BytomAssetKey:
		return BytomChainId
	default:
		panic(assetKey)
	}
}

var mainNetParams = litecoin.Params{
	Bech32HRPSegwit: "bn",
}
