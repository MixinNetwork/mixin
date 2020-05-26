// +build ed25519 !custom_alg

package bitcoin

import "github.com/MixinNetwork/mixin/crypto/ed25519"

func init() {
	ed25519.Load()
}
