package common

import (
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/domains/arweave"
	"github.com/MixinNetwork/mixin/domains/bch"
	"github.com/MixinNetwork/mixin/domains/binance"
	"github.com/MixinNetwork/mixin/domains/bitcoin"
	"github.com/MixinNetwork/mixin/domains/bsv"
	"github.com/MixinNetwork/mixin/domains/cosmos"
	"github.com/MixinNetwork/mixin/domains/dash"
	"github.com/MixinNetwork/mixin/domains/decred"
	"github.com/MixinNetwork/mixin/domains/dfinity"
	"github.com/MixinNetwork/mixin/domains/dogecoin"
	"github.com/MixinNetwork/mixin/domains/eos"
	"github.com/MixinNetwork/mixin/domains/etc"
	"github.com/MixinNetwork/mixin/domains/ethereum"
	"github.com/MixinNetwork/mixin/domains/filecoin"
	"github.com/MixinNetwork/mixin/domains/handshake"
	"github.com/MixinNetwork/mixin/domains/horizen"
	"github.com/MixinNetwork/mixin/domains/kusama"
	"github.com/MixinNetwork/mixin/domains/litecoin"
	"github.com/MixinNetwork/mixin/domains/mobilecoin"
	"github.com/MixinNetwork/mixin/domains/monero"
	"github.com/MixinNetwork/mixin/domains/namecoin"
	"github.com/MixinNetwork/mixin/domains/nervos"
	"github.com/MixinNetwork/mixin/domains/polkadot"
	"github.com/MixinNetwork/mixin/domains/ravencoin"
	"github.com/MixinNetwork/mixin/domains/ripple"
	"github.com/MixinNetwork/mixin/domains/siacoin"
	"github.com/MixinNetwork/mixin/domains/solana"
	"github.com/MixinNetwork/mixin/domains/stellar"
	"github.com/MixinNetwork/mixin/domains/tezos"
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
	case etc.EthereumClassicChainId:
		return etc.VerifyAssetKey(a.AssetKey)
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
	case ravencoin.RavencoinChainId:
		return ravencoin.VerifyAssetKey(a.AssetKey)
	case namecoin.NamecoinChainId:
		return namecoin.VerifyAssetKey(a.AssetKey)
	case dash.DashChainId:
		return dash.VerifyAssetKey(a.AssetKey)
	case decred.DecredChainId:
		return decred.VerifyAssetKey(a.AssetKey)
	case bch.BitcoinCashChainId:
		return bch.VerifyAssetKey(a.AssetKey)
	case bsv.BitcoinSVChainId:
		return bsv.VerifyAssetKey(a.AssetKey)
	case handshake.HandshakenChainId:
		return handshake.VerifyAssetKey(a.AssetKey)
	case nervos.NervosChainId:
		return nervos.VerifyAssetKey(a.AssetKey)
	case siacoin.SiacoinChainId:
		return siacoin.VerifyAssetKey(a.AssetKey)
	case filecoin.FilecoinChainId:
		return filecoin.VerifyAssetKey(a.AssetKey)
	case solana.SolanaChainId:
		return solana.VerifyAssetKey(a.AssetKey)
	case polkadot.PolkadotChainId:
		return polkadot.VerifyAssetKey(a.AssetKey)
	case kusama.KusamaChainId:
		return kusama.VerifyAssetKey(a.AssetKey)
	case ripple.RippleChainId:
		return ripple.VerifyAssetKey(a.AssetKey)
	case stellar.StellarChainId:
		return stellar.VerifyAssetKey(a.AssetKey)
	case tezos.TezosChainId:
		return tezos.VerifyAssetKey(a.AssetKey)
	case eos.EOSChainId:
		return eos.VerifyAssetKey(a.AssetKey)
	case tron.TronChainId:
		return tron.VerifyAssetKey(a.AssetKey)
	case mobilecoin.MobileCoinChainId:
		return mobilecoin.VerifyAssetKey(a.AssetKey)
	case cosmos.CosmosChainId:
		return cosmos.VerifyAssetKey(a.AssetKey)
	case binance.BinanceChainId:
		return binance.VerifyAssetKey(a.AssetKey)
	case arweave.ArweaveChainId:
		return arweave.VerifyAssetKey(a.AssetKey)
	case dfinity.DfinityChainId:
		return dfinity.VerifyAssetKey(a.AssetKey)
	default:
		return fmt.Errorf("invalid chain id %s", a.ChainId)
	}
}

func (a *Asset) AssetId() crypto.Hash {
	switch a.ChainId {
	case ethereum.EthereumChainId:
		return ethereum.GenerateAssetId(a.AssetKey)
	case etc.EthereumClassicChainId:
		return etc.GenerateAssetId(a.AssetKey)
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
	case ravencoin.RavencoinChainId:
		return ravencoin.GenerateAssetId(a.AssetKey)
	case namecoin.NamecoinChainId:
		return namecoin.GenerateAssetId(a.AssetKey)
	case dash.DashChainId:
		return dash.GenerateAssetId(a.AssetKey)
	case decred.DecredChainId:
		return decred.GenerateAssetId(a.AssetKey)
	case bch.BitcoinCashChainId:
		return bch.GenerateAssetId(a.AssetKey)
	case bsv.BitcoinSVChainId:
		return bsv.GenerateAssetId(a.AssetKey)
	case handshake.HandshakenChainId:
		return handshake.GenerateAssetId(a.AssetKey)
	case nervos.NervosChainId:
		return nervos.GenerateAssetId(a.AssetKey)
	case siacoin.SiacoinChainId:
		return siacoin.GenerateAssetId(a.AssetKey)
	case filecoin.FilecoinChainId:
		return filecoin.GenerateAssetId(a.AssetKey)
	case solana.SolanaChainId:
		return solana.GenerateAssetId(a.AssetKey)
	case polkadot.PolkadotChainId:
		return polkadot.GenerateAssetId(a.AssetKey)
	case kusama.KusamaChainId:
		return kusama.GenerateAssetId(a.AssetKey)
	case ripple.RippleChainId:
		return ripple.GenerateAssetId(a.AssetKey)
	case stellar.StellarChainId:
		return stellar.GenerateAssetId(a.AssetKey)
	case tezos.TezosChainId:
		return tezos.GenerateAssetId(a.AssetKey)
	case eos.EOSChainId:
		return eos.GenerateAssetId(a.AssetKey)
	case tron.TronChainId:
		return tron.GenerateAssetId(a.AssetKey)
	case mobilecoin.MobileCoinChainId:
		return mobilecoin.GenerateAssetId(a.AssetKey)
	case cosmos.CosmosChainId:
		return cosmos.GenerateAssetId(a.AssetKey)
	case binance.BinanceChainId:
		return binance.GenerateAssetId(a.AssetKey)
	case arweave.ArweaveChainId:
		return arweave.GenerateAssetId(a.AssetKey)
	case dfinity.DfinityChainId:
		return dfinity.GenerateAssetId(a.AssetKey)
	default:
		return crypto.Hash{}
	}
}

func (a *Asset) FeeAssetId() crypto.Hash {
	switch a.ChainId {
	case ethereum.EthereumChainId:
		return ethereum.EthereumChainId
	case etc.EthereumClassicChainId:
		return etc.EthereumClassicChainId
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
	case ravencoin.RavencoinChainId:
		return ravencoin.RavencoinChainId
	case namecoin.NamecoinChainId:
		return namecoin.NamecoinChainId
	case dash.DashChainId:
		return dash.DashChainId
	case decred.DecredChainId:
		return decred.DecredChainId
	case bch.BitcoinCashChainId:
		return bch.BitcoinCashChainId
	case bsv.BitcoinSVChainId:
		return bsv.BitcoinSVChainId
	case handshake.HandshakenChainId:
		return handshake.HandshakenChainId
	case nervos.NervosChainId:
		return nervos.NervosChainId
	case siacoin.SiacoinChainId:
		return siacoin.SiacoinChainId
	case filecoin.FilecoinChainId:
		return filecoin.FilecoinChainId
	case solana.SolanaChainId:
		return solana.SolanaChainId
	case polkadot.PolkadotChainId:
		return polkadot.PolkadotChainId
	case kusama.KusamaChainId:
		return kusama.KusamaChainId
	case ripple.RippleChainId:
		return ripple.RippleChainId
	case stellar.StellarChainId:
		return stellar.StellarChainId
	case tezos.TezosChainId:
		return tezos.TezosChainId
	case eos.EOSChainId:
		return eos.EOSChainId
	case tron.TronChainId:
		return tron.TronChainId
	case mobilecoin.MobileCoinChainId:
		return mobilecoin.MobileCoinChainId
	case cosmos.CosmosChainId:
		return cosmos.CosmosChainId
	case binance.BinanceChainId:
		return binance.BinanceChainId
	case arweave.ArweaveChainId:
		return arweave.ArweaveChainId
	case dfinity.DfinityChainId:
		return dfinity.DfinityChainId
	}
	return crypto.Hash{}
}
