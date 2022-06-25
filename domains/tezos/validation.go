package tezos

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcd/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

var (
	TezosChainBase string
	TezosChainId   crypto.Hash

	tz1Prefix = []byte{6, 161, 159}
	opPrefix  = []byte{5, 116}
)

func init() {
	TezosChainBase = "5649ca42-eb5f-4c0e-ae28-d9a4e77eded3"
	TezosChainId = crypto.NewHash([]byte(TezosChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == TezosChainBase {
		return nil
	}
	return fmt.Errorf("invalid tezos asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid tezos address %s", address)
	}
	address = strings.TrimSpace(address)
	decoded, err := decodeAddress(address)
	if err != nil {
		return fmt.Errorf("invalid tezos address %s %s", address, err)
	}
	xtzAddress := CheckEncode(decoded, tz1Prefix[:])
	if xtzAddress != address {
		return fmt.Errorf("invalid tezos address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if strings.TrimSpace(hash) != hash {
		return fmt.Errorf("invalid tezos transaction hash %s", hash)
	}
	decoded, p, err := CheckDecode(hash, len(opPrefix))
	if err != nil {
		return fmt.Errorf("invalid tezos transaction hash %s %s", hash, err.Error())
	}
	if bytes.Compare(p, opPrefix) != 0 {
		return fmt.Errorf("invalid tezos transaction hash %s", hash)
	}
	if len(decoded) != 32 {
		return fmt.Errorf("invalid tezos transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case TezosChainBase:
		return TezosChainId
	default:
		panic(assetKey)
	}
}

func decodeAddress(address string) ([]byte, error) {
	decoded, netID, err := CheckDecode(address, len(tz1Prefix))
	if err != nil {
		return nil, err
	}
	if len(decoded) != ripemd160.Size {
		return nil, fmt.Errorf("decode address is of unknown size %d", len(decoded))
	}
	if bytes.Compare(netID, tz1Prefix) != 0 {
		return nil, fmt.Errorf("decode address is of unknown prefix %x", netID)
	}
	return decoded, nil
}

// ErrChecksum indicates that the checksum of a check-encoded string does not verify against
// the checksum.
var ErrChecksum = errors.New("checksum error")

// ErrInvalidFormat indicates that the check-encoded string has an invalid format.
var ErrInvalidFormat = errors.New("invalid format: version and/or checksum bytes missing")

// checksum: first four bytes of sha256^2
func checksum(input []byte) (cksum [4]byte) {
	h := sha256.Sum256(input)
	h2 := sha256.Sum256(h[:])
	copy(cksum[:], h2[:4])
	return
}

// CheckEncode prepends two version bytes and appends a four byte checksum.
func CheckEncode(input []byte, version []byte) string {
	b := make([]byte, 0, len(version)+len(input)+4)
	b = append(b, version[:]...)
	b = append(b, input[:]...)
	cksum := checksum(b)
	b = append(b, cksum[:]...)
	return base58.Encode(b)
}

// CheckDecode decodes a string that was encoded with CheckEncode and verifies
// the checksum.
func CheckDecode(input string, pl int) (result []byte, prefix []byte, err error) {
	decoded := base58.Decode(input)
	if len(decoded) < pl+4 {
		return nil, nil, ErrInvalidFormat
	}
	for i := 0; i < pl; i++ {
		prefix = append(prefix, decoded[i])
	}
	var cksum [4]byte
	copy(cksum[:], decoded[len(decoded)-4:])
	if checksum(decoded[:len(decoded)-4]) != cksum {
		return nil, nil, ErrChecksum
	}
	payload := decoded[pl : len(decoded)-4]
	result = append(result, payload...)
	return
}
