package ed25519

import (
	"errors"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/crypto/ed25519/edwards25519"
	"golang.org/x/crypto/sha3"
)

type keyFactory struct{}

func NewPrivateKeyFromSeed(seed []byte) (*Key, error) {
	var (
		src [64]byte
		out [32]byte
	)
	if len(seed) != len(src) {
		return nil, errors.New("invalid seed")
	}
	copy(src[:], seed)
	edwards25519.ScReduce(&out, &src)
	key := Key(out)
	if !key.CheckScalar() {
		return nil, errors.New("invalid key: check scalar failed")
	}
	return &key, nil
}

func NewPrivateKeyFromSeedPanic(seed []byte) *Key {
	key, err := NewPrivateKeyFromSeed(seed)
	if err != nil {
		panic(err)
	}
	return key
}

func (f keyFactory) NewPrivateKeyFromSeed(seed []byte) (crypto.PrivateKey, error) {
	return NewPrivateKeyFromSeed(seed)
}

func (f keyFactory) NewPrivateKeyFromSeedPanic(seed []byte) crypto.PrivateKey {
	return NewPrivateKeyFromSeedPanic(seed)
}

func (f keyFactory) PrivateKeyFromKey(k crypto.Key) (crypto.PrivateKey, error) {
	key := KeyFromCryptoKey(k)
	if !key.CheckScalar() {
		return nil, errors.New("invalid key: check scalar failed")
	}
	return key, nil
}

func (f keyFactory) PublicKeyFromKey(k crypto.Key) (crypto.PublicKey, error) {
	key := KeyFromCryptoKey(k)
	if !key.CheckKey() {
		return nil, errors.New("invalid key: check key failed")
	}
	return key, nil
}

func (f keyFactory) PrivateKeyFromBytes(b []byte) (crypto.PrivateKey, error) {
	return KeyFromBytes(b)
}
func (f keyFactory) PublicKeyFromBytes(b []byte) (crypto.PublicKey, error) {
	return KeyFromBytes(b)
}

func Load() {
	crypto.SetKeyFactory(keyFactory{})
	crypto.SetHashFunc(sha3.Sum256)
}
