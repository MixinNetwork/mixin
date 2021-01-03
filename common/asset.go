package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/bitcoin"
	"github.com/MixinNetwork/mixin/domains/eos"
	"github.com/MixinNetwork/mixin/domains/ethereum"
	"github.com/MixinNetwork/mixin/domains/monero"
	"github.com/MixinNetwork/mixin/domains/siacoin"
	"github.com/MixinNetwork/mixin/domains/zcash"
)

var (
	XINAssetId crypto.Hash
)

type Asset struct {
	ChainId  crypto.Hash
	AssetKey string
}

func init() {
	XINAssetId = crypto.NewHash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8"))
}

func (a *Asset) Verify() error {
	switch a.ChainId {
	case ethereum.EthereumChainId:
		return ethereum.VerifyAssetKey(a.AssetKey)
	case bitcoin.BitcoinChainId:
		return bitcoin.VerifyAssetKey(a.AssetKey)
	case monero.MoneroChainId:
		return monero.VerifyAssetKey(a.AssetKey)
	case zcash.ZcashChainId:
		return zcash.VerifyAssetKey(a.AssetKey)
	case siacoin.SiacoinChainId:
		return siacoin.VerifyAssetKey(a.AssetKey)
	case eos.EOSChainId:
		return eos.VerifyAssetKey(a.AssetKey)
	default:
		return fmt.Errorf("invalid chain id %s", a.ChainId)
	}
}

func (a *Asset) AssetId() crypto.Hash {
	switch a.ChainId {
	case ethereum.EthereumChainId:
		return ethereum.GenerateAssetId(a.AssetKey)
	case bitcoin.BitcoinChainId:
		return bitcoin.GenerateAssetId(a.AssetKey)
	case monero.MoneroChainId:
		return monero.GenerateAssetId(a.AssetKey)
	case zcash.ZcashChainId:
		return zcash.GenerateAssetId(a.AssetKey)
	case siacoin.SiacoinChainId:
		return siacoin.GenerateAssetId(a.AssetKey)
	case eos.EOSChainId:
		return eos.GenerateAssetId(a.AssetKey)
	default:
		return crypto.Hash{}
	}
}

func (a *Asset) FeeAssetId() crypto.Hash {
	switch a.ChainId {
	case ethereum.EthereumChainId:
		return ethereum.EthereumChainId
	case bitcoin.BitcoinChainId:
		return bitcoin.BitcoinChainId
	case monero.MoneroChainId:
		return monero.MoneroChainId
	case zcash.ZcashChainId:
		return zcash.ZcashChainId
	case siacoin.SiacoinChainId:
		return siacoin.SiacoinChainId
	case eos.EOSChainId:
		return eos.EOSChainId
	}
	return crypto.Hash{}
}
