package external

import (
	"fmt"

	bitcoinCash "mixin.one/blockchain/external/bitcoin-cash/api"
	bitcoin "mixin.one/blockchain/external/bitcoin/api"
	ethereumClassic "mixin.one/blockchain/external/ethereum-classic/api"
	ethereum "mixin.one/blockchain/external/ethereum/api"
	litecoin "mixin.one/blockchain/external/litecoin/api"
	ripple "mixin.one/blockchain/external/ripple/api"
	siacoin "mixin.one/blockchain/external/siacoin/api"
)

func LocalGenerateKey(chainId string) (string, string, error) {
	switch chainId {
	case RippleChainId:
		return ripple.LocalGenerateKey()
	case SiacoinChainId:
		return siacoin.LocalGenerateKey()
	case EthereumChainId:
		return ethereum.LocalGenerateKey()
	case EthereumClassicChainId:
		return ethereumClassic.LocalGenerateKey()
	case BitcoinChainId:
		return bitcoin.LocalGenerateKey()
	case BitcoinCashChainId:
		return bitcoinCash.LocalGenerateKey()
	case LitecoinChainId:
		return litecoin.LocalGenerateKey()
	}
	return "", "", fmt.Errorf("unsupported chain id %s", chainId)
}

func NormalizeAddress(chainId, address string) (string, error) {
	switch chainId {
	case RippleChainId:
		return ripple.LocalNormalizePublicKey(address)
	case SiacoinChainId:
		return siacoin.LocalNormalizePublicKey(address)
	case EthereumChainId:
		return ethereum.LocalNormalizePublicKey(address)
	case EthereumClassicChainId:
		return ethereumClassic.LocalNormalizePublicKey(address)
	case BitcoinChainId:
		return bitcoin.LocalNormalizePublicKey(address)
	case BitcoinCashChainId:
		return bitcoinCash.LocalNormalizePublicKey(address)
	case LitecoinChainId:
		return litecoin.LocalNormalizePublicKey(address)
	}
	return "", fmt.Errorf("unsupported chain %s", chainId)
}
