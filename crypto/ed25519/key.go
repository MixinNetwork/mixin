package ed25519

import (
	"errors"

	"github.com/MixinNetwork/mixin/crypto"
)

type Key crypto.Key

func (k Key) Key() crypto.Key {
	return crypto.Key(k)
}

func (k Key) String() string {
	return crypto.Key(k).String()
}

func KeyFromBytes(b []byte) (*Key, error) {
	var key crypto.Key
	if len(b) != len(key) {
		return nil, errors.New("invalid key length")
	}

	copy(key[:], b)
	return KeyFromCryptoKey(key), nil
}

func KeyFromCryptoKey(k crypto.Key) *Key {
	var key = Key(k)
	return &key
}
