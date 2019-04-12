package common

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/gofrs/uuid"
)

var (
	EthereumChainBase string
	EthereumChainId   crypto.Hash

	XINAssetId crypto.Hash
)

type Asset struct {
	ChainId  crypto.Hash
	AssetKey string
}

func init() {
	EthereumChainBase = "43d61dcd-e413-450d-80b8-101d5e903357"
	EthereumChainId = crypto.NewHash([]byte(EthereumChainBase))

	XINAssetId = crypto.NewHash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8"))
}

func (a *Asset) Verify() error {
	switch a.ChainId {
	case EthereumChainId:
		if len(a.AssetKey) != 42 {
			return fmt.Errorf("invalid ethereum asset key %s", a.AssetKey)
		}
		if !strings.HasPrefix(a.AssetKey, "0x") {
			return fmt.Errorf("invalid ethereum asset key %s", a.AssetKey)
		}
		if a.AssetKey != strings.ToLower(a.AssetKey) {
			return fmt.Errorf("invalid ethereum asset key %s", a.AssetKey)
		}
		key, err := hex.DecodeString(a.AssetKey[2:])
		if err != nil {
			return fmt.Errorf("invalid ethereum asset key %s %s", a.AssetKey, err.Error())
		}
		if len(key) != 20 {
			return fmt.Errorf("invalid ethereum asset key %s", a.AssetKey)
		}
		return nil
	default:
		return fmt.Errorf("invalid chain id %s", a.ChainId)
	}
}

func (a *Asset) AssetId() crypto.Hash {
	var chainBase string
	switch a.ChainId {
	case EthereumChainId:
		chainBase = EthereumChainBase
		if a.AssetKey == "0x0000000000000000000000000000000000000000" {
			return EthereumChainId
		}
	default:
		return crypto.Hash{}
	}

	h := md5.New()
	io.WriteString(h, chainBase)
	io.WriteString(h, a.AssetKey)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	id := uuid.FromBytesOrNil(sum).String()
	return crypto.NewHash([]byte(id))
}

func (a *Asset) FeeAssetId() crypto.Hash {
	switch a.ChainId {
	case EthereumChainId:
		return EthereumChainId
	}
	return crypto.Hash{}
}
