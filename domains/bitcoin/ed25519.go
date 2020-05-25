// +build ed25519 !custom_alg

package bitcoin

import (
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/crypto/ed25519"
)

func init() {
	ed25519.Load()

	BitcoinChainId = crypto.NewHash([]byte(BitcoinChainAssetKey))
	BitcoinOmniUSDTId = crypto.NewHash([]byte(BitcoinOmniUSDTAssetKey))
}
