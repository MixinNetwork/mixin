// https://github.com/dedis/kyber/blob/master/xof/blake2xb/blake.go
// Deterministic test reader based on the Blake2xb construction.
package crypto

import (
	"golang.org/x/crypto/blake2b"
)

type xof struct {
	impl blake2b.XOF
}

// NewBlake2bXOF creates a deterministic reader using the Blake2b hash.
func NewBlake2bXOF(seed []byte) *xof {
	seed1 := seed
	var seed2 []byte
	if len(seed) > blake2b.Size {
		seed1 = seed[0:blake2b.Size]
		seed2 = seed[blake2b.Size:]
	}
	b, err := blake2b.NewXOF(blake2b.OutputLengthUnknown, seed1)
	if err != nil {
		panic("blake2b.NewXOF should not return error: " + err.Error())
	}

	if seed2 != nil {
		_, err := b.Write(seed2)
		if err != nil {
			panic("blake2b.XOF.Write should not return error: " + err.Error())
		}
	}
	return &xof{impl: b}
}

func (x *xof) Read(dst []byte) (int, error) {
	return x.impl.Read(dst)
}
