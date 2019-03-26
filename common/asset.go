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
	CNBAssetId crypto.Hash
)

type Asset struct {
	ChainId  crypto.Hash
	AssetKey string
}

func init() {
	EthereumChainBase = "43d61dcd-e413-450d-80b8-101d5e903357"
	EthereumChainId = crypto.NewHash([]byte(EthereumChainBase))

	XINAssetId = crypto.NewHash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8"))
	CNBAssetId = crypto.NewHash([]byte("965e5c6e-434c-3fa9-b780-c50f43cd955c"))
}

func (d *DepositData) validateEthereumAssetInput() error {
	if d.Chain != EthereumChainId {
		return fmt.Errorf("invalid chain %s", d.Chain)
	}
	if id := d.Asset().AssetId(); id != XINAssetId && id != CNBAssetId {
		return fmt.Errorf("invalid asset %s %s", d.AssetKey, id)
	}
	if d.Amount.Sign() <= 0 {
		return fmt.Errorf("invalid amount %s", d.Amount.String())
	}
	if d.OutputIndex > 256 {
		return fmt.Errorf("invalid output index %d", d.OutputIndex)
	}
	if len(d.TransactionHash) != 66 || !strings.HasPrefix(d.TransactionHash, "0x") {
		return fmt.Errorf("invalid transaction hash %s", d.TransactionHash)
	}
	h, err := hex.DecodeString(d.TransactionHash[2:])
	if err != nil || len(h) != 32 {
		return fmt.Errorf("invalid transaction hash %s %s", d.TransactionHash, err)
	}
	return nil
}

func (d *DepositData) UniqueKey() crypto.Hash {
	index := fmt.Sprintf("%s:%d", d.TransactionHash, d.OutputIndex)
	return crypto.NewHash([]byte(index)).ForNetwork(d.Chain)
}

func (d *DepositData) Asset() *Asset {
	return &Asset{
		ChainId:  d.Chain,
		AssetKey: d.AssetKey,
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
