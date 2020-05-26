package ed25519

import (
	"github.com/MixinNetwork/mixin/crypto"
)

type Key crypto.Key

func (k Key) Key() crypto.Key {
	return crypto.Key(k)
}

func (k Key) String() string {
	return crypto.Key(k).String()
}
