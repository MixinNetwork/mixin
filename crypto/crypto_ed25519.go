// +build ed25519 !custom_alg

package crypto

import "golang.org/x/crypto/sha3"

const (
	KeySize      = 32
	ResponseSize = 32
)

type Key [KeySize]byte
type Response [ResponseSize]byte
type Commitment Key

func init() {
	hashFunc = sha3.Sum256
}
