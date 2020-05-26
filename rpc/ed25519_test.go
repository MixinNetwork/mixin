// +build ed25519 !custom_alg

package rpc

import "github.com/MixinNetwork/mixin/crypto/ed25519"

func init() {
	ed25519.Load()
}
