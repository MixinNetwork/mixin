// +build ed25519 !custom_alg

package storage

import "github.com/MixinNetwork/mixin/crypto/ed25519"

var configFilePath = "../config/config.example.toml"

func init() {
	ed25519.Load()
}
