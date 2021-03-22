package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/bitcoin"
	"github.com/MixinNetwork/mixin/domains/cosmos"
	"github.com/MixinNetwork/mixin/domains/dogecoin"
	"github.com/MixinNetwork/mixin/domains/eos"
	"github.com/MixinNetwork/mixin/domains/ethereum"
	"github.com/MixinNetwork/mixin/domains/horizen"
	"github.com/MixinNetwork/mixin/domains/kusama"
	"github.com/MixinNetwork/mixin/domains/litecoin"
	"github.com/MixinNetwork/mixin/domains/mobilecoin"
	"github.com/MixinNetwork/mixin/domains/monero"
	"github.com/MixinNetwork/mixin/domains/polkadot"
	"github.com/MixinNetwork/mixin/domains/siacoin"
	"github.com/MixinNetwork/mixin/domains/tron"
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
	case horizen.HorizenChainId:
		return horizen.VerifyAssetKey(a.AssetKey)
	case litecoin.LitecoinChainId:
		return litecoin.VerifyAssetKey(a.AssetKey)
	case dogecoin.DogecoinChainId:
		return dogecoin.VerifyAssetKey(a.AssetKey)
	case siacoin.SiacoinChainId:
		return siacoin.VerifyAssetKey(a.AssetKey)
	case polkadot.PolkadotChainId:
		return polkadot.VerifyAssetKey(a.AssetKey)
	case kusama.KusamaChainId:
		return kusama.VerifyAddress(a.AssetKey)
	case eos.EOSChainId:
		return eos.VerifyAssetKey(a.AssetKey)
	case tron.TronChainId:
		return tron.VerifyAssetKey(a.AssetKey)
	case mobilecoin.MobileCoinChainId:
		return mobilecoin.VerifyAssetKey(a.AssetKey)
	case cosmos.CosmosChainId:
		return cosmos.VerifyAssetKey(a.AssetKey)
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
	case horizen.HorizenChainId:
		return horizen.GenerateAssetId(a.AssetKey)
	case litecoin.LitecoinChainId:
		return litecoin.GenerateAssetId(a.AssetKey)
	case dogecoin.DogecoinChainId:
		return dogecoin.GenerateAssetId(a.AssetKey)
	case siacoin.SiacoinChainId:
		return siacoin.GenerateAssetId(a.AssetKey)
	case polkadot.PolkadotChainId:
		return polkadot.GenerateAssetId(a.AssetKey)
	case kusama.KusamaChainId:
		return kusama.GenerateAssetId(a.AssetKey)
	case eos.EOSChainId:
		return eos.GenerateAssetId(a.AssetKey)
	case tron.TronChainId:
		return tron.GenerateAssetId(a.AssetKey)
	case mobilecoin.MobileCoinChainId:
		return mobilecoin.GenerateAssetId(a.AssetKey)
	case cosmos.CosmosChainId:
		return cosmos.GenerateAssetId(a.AssetKey)
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
	case horizen.HorizenChainId:
		return horizen.HorizenChainId
	case litecoin.LitecoinChainId:
		return litecoin.LitecoinChainId
	case dogecoin.DogecoinChainId:
		return dogecoin.DogecoinChainId
	case siacoin.SiacoinChainId:
		return siacoin.SiacoinChainId
	case polkadot.PolkadotChainId:
		return polkadot.PolkadotChainId
	case kusama.KusamaChainId:
		return kusama.KusamaChainId
	case eos.EOSChainId:
		return eos.EOSChainId
	case tron.TronChainId:
		return tron.TronChainId
	case mobilecoin.MobileCoinChainId:
		return mobilecoin.MobileCoinChainId
	case cosmos.CosmosChainId:
		return cosmos.CosmosChainId
	}
	return crypto.Hash{}
}
