// +build ed25519 !custom_alg

package ethereum

import (
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/crypto/ed25519"
)

func init() {
	ed25519.Load()

	EthereumChainBase = "43d61dcd-e413-450d-80b8-101d5e903357"
	EthereumChainId = crypto.NewHash([]byte(EthereumChainBase))
}
