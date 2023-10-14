package common

import (
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	XINAsset        *Asset
	XINAssetId      crypto.Hash
	BitcoinAssetId  crypto.Hash
	EthereumAssetId crypto.Hash
)

type Asset struct {
	Chain    crypto.Hash
	AssetKey string
}

func init() {
	XINAssetId = crypto.Sha256Hash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8"))
	BitcoinAssetId = crypto.Sha256Hash([]byte("c6d0c728-2624-429b-8e0d-d9d19b6592fa"))
	EthereumAssetId = crypto.Sha256Hash([]byte("43d61dcd-e413-450d-80b8-101d5e903357"))
	XINAsset = &Asset{Chain: EthereumAssetId, AssetKey: "0xa974c709cfb4566686553a20790685a47aceaa33"}
}

func (a *Asset) Verify() error {
	if !a.Chain.HasValue() {
		return fmt.Errorf("invalid asset chain %v", *a)
	}
	if strings.TrimSpace(a.AssetKey) != a.AssetKey || len(a.AssetKey) == 0 {
		return fmt.Errorf("invalid asset key %v", *a)
	}
	return nil
}
