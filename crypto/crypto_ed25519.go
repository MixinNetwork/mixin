// +build ed25519 !custom_alg

package crypto

const KeySize = 32

type Key [KeySize]byte
type Response [32]byte
type Commitment Key
