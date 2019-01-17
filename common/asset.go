package common

import "github.com/MixinNetwork/mixin/crypto"

var (
	XINAssetId      crypto.Hash
	BitcoinChainId  crypto.Hash
	EthereumChainId crypto.Hash
)

func init() {
	XINAssetId = crypto.NewHash([]byte("c94ac88f-4671-3976-b60a-09064f1811e8"))

	BitcoinChainId = crypto.NewHash([]byte("c6d0c728-2624-429b-8e0d-d9d19b6592fa"))
	EthereumChainId = crypto.NewHash([]byte("43d61dcd-e413-450d-80b8-101d5e903357"))
}
