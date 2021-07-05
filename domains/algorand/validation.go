package algorand

import (
	"bytes"
	"crypto/sha512"
	"encoding/base32"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	AlgorandChainBase string
	AlgorandChainId   crypto.Hash
)

func init() {
	AlgorandChainBase = "706b6f84-3333-4e55-8e89-275e71ce9803"
	AlgorandChainId = crypto.NewHash([]byte(AlgorandChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == AlgorandChainBase {
		return nil
	}
	return fmt.Errorf("invalid algorand asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid algorand address %s", address)
	}
	addr, err := decodeAddress(address)
	if err != nil {
		return fmt.Errorf("invalid algorand address %s", address)
	}
	if addr.String() != address {
		return fmt.Errorf("invalid algorand address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 52 {
		return fmt.Errorf("invalid algorand transaction hash %s", hash)
	}
	if strings.ToUpper(hash) != hash {
		return fmt.Errorf("invalid algorand transaction hash %s", hash)
	}
	h, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid algorand transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid algorand transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case AlgorandChainBase:
		return AlgorandChainId
	default:
		panic(assetKey)
	}
}

const (
	checksumLenBytes = 4
	hashLenBytes     = sha512.Size256
)

// Address represents an Algorand address.
type Address [hashLenBytes]byte

// String grabs a human-readable representation of the address. This
// representation includes a 4-byte checksum.
func (a Address) String() string {
	// Compute the checksum
	checksumHash := sha512.Sum512_256(a[:])
	checksumLenBytes := checksumHash[hashLenBytes-checksumLenBytes:]

	// Append the checksum and encode as base32
	checksumAddress := append(a[:], checksumLenBytes...)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(checksumAddress)
}

func decodeAddress(addr string) (a Address, err error) {
	// Interpret the address as base32
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(addr)
	if err != nil {
		return
	}

	// Ensure the decoded address is the correct length
	if len(decoded) != len(a)+checksumLenBytes {
		err = fmt.Errorf("invalid address len %d", len(decoded))
		return
	}

	// Split into address + checksum
	addressBytes := decoded[:len(a)]
	checksumBytes := decoded[len(a):]

	// Compute the expected checksum
	checksumHash := sha512.Sum512_256(addressBytes)
	expectedChecksumBytes := checksumHash[hashLenBytes-checksumLenBytes:]

	// Check the checksum
	if !bytes.Equal(expectedChecksumBytes, checksumBytes) {
		err = fmt.Errorf("invalid address checksum")
		return
	}

	// Checksum is good, copy address bytes into output
	copy(a[:], addressBytes)
	return a, nil
}
