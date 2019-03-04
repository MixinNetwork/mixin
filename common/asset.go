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
	if d.AssetId() != XINAssetId && d.AssetId() != CNBAssetId {
		return fmt.Errorf("invalid asset %s %s", d.AssetKey, d.AssetId())
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

func (d *DepositData) AssetId() crypto.Hash {
	var chainBase string
	switch d.Chain {
	case EthereumChainId:
		chainBase = EthereumChainBase
	default:
		return crypto.Hash{}
	}

	h := md5.New()
	io.WriteString(h, chainBase)
	io.WriteString(h, d.AssetKey)
	sum := h.Sum(nil)
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	id := uuid.FromBytesOrNil(sum).String()
	return crypto.NewHash([]byte(id))
}
