package ethereum

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofrs/uuid"
)

var (
	EthereumChainBase string
	EthereumChainId   crypto.Hash
)

func init() {
	EthereumChainBase = "43d61dcd-e413-450d-80b8-101d5e903357"
	EthereumChainId = crypto.NewHash([]byte(EthereumChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if len(assetKey) != 42 {
		return fmt.Errorf("invalid ethereum asset key %s", assetKey)
	}
	if !strings.HasPrefix(assetKey, "0x") {
		return fmt.Errorf("invalid ethereum asset key %s", assetKey)
	}
	if assetKey != strings.ToLower(assetKey) {
		return fmt.Errorf("invalid ethereum asset key %s", assetKey)
	}
	k, err := hex.DecodeString(assetKey[2:])
	if err != nil {
		return fmt.Errorf("invalid ethereum asset key %s %s", assetKey, err.Error())
	}
	if len(k) != 20 {
		return fmt.Errorf("invalid ethereum asset key %s", assetKey)
	}
	return nil
}

func VerifyAddress(address string) error {
	if len(address) != 42 {
		return fmt.Errorf("invalid ethereum address %s", address)
	}
	if !strings.HasPrefix(address, "0x") {
		return fmt.Errorf("invalid ethereum address %s", address)
	}
	form, err := formatAddress(address)
	if err != nil {
		return fmt.Errorf("invalid ethereum address %s", address)
	}
	if form != address {
		return fmt.Errorf("invalid ethereum address %s", address)
	}
	a, err := hex.DecodeString(address[2:])
	if err != nil {
		return fmt.Errorf("invalid ethereum address %s %s", address, err.Error())
	}
	if len(a) != 20 {
		return fmt.Errorf("invalid ethereum address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 66 {
		return fmt.Errorf("invalid ethereum transaction hash %s", hash)
	}
	if !strings.HasPrefix(hash, "0x") {
		return fmt.Errorf("invalid ethereum transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid ethereum transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash[2:])
	if err != nil {
		return fmt.Errorf("invalid ethereum transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid ethereum transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	err := VerifyAssetKey(assetKey)
	if err != nil {
		panic(assetKey)
	}

	if assetKey == "0x0000000000000000000000000000000000000000" {
		return EthereumChainId
	}

	h := md5.New()
	io.WriteString(h, EthereumChainBase)
	io.WriteString(h, assetKey)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	id := uuid.FromBytesOrNil(sum).String()
	return crypto.NewHash([]byte(id))
}

func formatAddress(to string) (string, error) {
	var bytesto [20]byte
	_bytesto, err := hex.DecodeString(to[2:])
	if err != nil {
		return "", err
	}
	copy(bytesto[:], _bytesto)
	address := common.Address(bytesto)
	return address.Hex(), nil
}
