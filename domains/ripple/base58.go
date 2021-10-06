package ripple

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"
)

// Purloined from https://github.com/conformal/btcutil/

var bigRadix = big.NewInt(58)
var bigZero = big.NewInt(0)

// Base58Decode decodes a modified base58 string to a byte slice and checks checksum.
func Base58Decode(b, alphabet string) ([]byte, error) {
	if len(b) < 5 {
		return nil, fmt.Errorf("Base58 string too short: %s", b)
	}
	answer := big.NewInt(0)
	j := big.NewInt(1)

	for i := len(b) - 1; i >= 0; i-- {
		tmp := strings.IndexAny(alphabet, string(b[i]))
		if tmp == -1 {
			return nil, fmt.Errorf("Bad Base58 string: %s", b)
		}
		idx := big.NewInt(int64(tmp))
		tmp1 := big.NewInt(0)
		tmp1.Mul(j, idx)

		answer.Add(answer, tmp1)
		j.Mul(j, bigRadix)
	}

	tmpval := answer.Bytes()

	var numZeros int
	for numZeros = 0; numZeros < len(b); numZeros++ {
		if b[numZeros] != alphabet[0] {
			break
		}
	}
	flen := numZeros + len(tmpval)
	val := make([]byte, flen, flen)
	copy(val[numZeros:], tmpval)

	// Check checksum
	checksum := DoubleSha256(val[0 : len(val)-4])
	expected := val[len(val)-4:]
	if !bytes.Equal(checksum[0:4], expected) {
		return nil, fmt.Errorf("Bad Base58 checksum: %v expected %v", checksum, expected)
	}
	return val, nil
}

// Base58Encode encodes a byte slice to a modified base58 string.
func Base58Encode(b []byte, alphabet string) string {
	checksum := DoubleSha256(b)
	b = append(b, checksum[0:4]...)
	x := new(big.Int)
	x.SetBytes(b)

	answer := make([]byte, 0)
	for x.Cmp(bigZero) > 0 {
		mod := new(big.Int)
		x.DivMod(x, bigRadix, mod)
		answer = append(answer, alphabet[mod.Int64()])
	}

	// leading zero bytes
	for _, i := range b {
		if i != 0 {
			break
		}
		answer = append(answer, alphabet[0])
	}

	// reverse
	alen := len(answer)
	for i := 0; i < alen/2; i++ {
		answer[i], answer[alen-1-i] = answer[alen-1-i], answer[i]
	}

	return string(answer)
}

func DoubleSha256(b []byte) []byte {
	hasher := sha256.New()
	hasher.Write(b)
	sha := hasher.Sum(nil)
	hasher.Reset()
	hasher.Write(sha)
	return hasher.Sum(nil)
}
