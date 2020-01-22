package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/bitcoin"
	"github.com/MixinNetwork/mixin/domains/ethereum"
)

var (
	EthereumChainId crypto.Hash
	BitcoinChainId  crypto.Hash

	XINAssetId crypto.Hash
)

type Asset struct {
	ChainId  crypto.Hash
	AssetKey string
}

func init() {
	EthereumChainId = crypto.NewHash([]byte("43d61dcd-e413-450d-80b8-101d5e903357"))
	BitcoinChainId = crypto.NewHash([]byte("c6d0c728-2624-429b-8e0d-d9d19b6592fa"))

	XINAssetId = crypto.NewHash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8"))
}

func (a *Asset) Verify() error {
	switch a.ChainId {
	case EthereumChainId:
		return ethereum.VerifyAssetKey(a.AssetKey)
	case BitcoinChainId:
		return bitcoin.VerifyAssetKey(a.AssetKey)
	default:
		return fmt.Errorf("invalid chain id %s", a.ChainId)
	}
}

func (a *Asset) AssetId() crypto.Hash {
	switch a.ChainId {
	case EthereumChainId:
		return ethereum.GenerateAssetId(a.AssetKey)
	case BitcoinChainId:
		return bitcoin.GenerateAssetId(a.AssetKey)
	default:
		return crypto.Hash{}
	}
}

func (a *Asset) FeeAssetId() crypto.Hash {
	switch a.ChainId {
	case EthereumChainId:
		return EthereumChainId
	}
	return crypto.Hash{}
}
