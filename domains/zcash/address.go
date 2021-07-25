package zcash

import (
	"crypto/sha256"
	"errors"

	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

type Params struct {
	PubKeyBase58Prefixes [2]byte
	ScriptBase58Prefixes [2]byte
}

func encodeAddress(hash160 []byte, netID [2]byte) string {
	return CheckEncode(hash160[:ripemd160.Size], netID)
}

type Address interface {
	EncodeAddress() string
}

type AddressPubKeyHash struct {
	hash  [ripemd160.Size]byte
	netID [2]byte
}

func (a *AddressPubKeyHash) EncodeAddress() string {
	return encodeAddress(a.hash[:], a.netID)
}

type AddressScriptHash struct {
	hash  [ripemd160.Size]byte
	netID [2]byte
}

// EncodeAddress returns the string encoding of a pay-to-script-hash
// address.  Part of the Address interface.
func (a *AddressScriptHash) EncodeAddress() string {
	return encodeAddress(a.hash[:], a.netID)
}

func DecodeAddress(address string, params *Params) (Address, error) {
	decoded, netID, err := CheckDecode(address)
	if err != nil {
		return nil, err
	}

	switch len(decoded) {
	case ripemd160.Size: // P2PKH or P2SH
		isP2PKH := netID == params.PubKeyBase58Prefixes
		isP2SH := netID == params.ScriptBase58Prefixes
		switch hash160 := decoded; {
		case isP2PKH && isP2SH:
			return nil, btcutil.ErrAddressCollision
		case isP2PKH:
			return newAddressPubKeyHash(hash160, netID)
		case isP2SH:
			return newAddressScriptHashFromHash(hash160, netID)
		default:
			return nil, btcutil.ErrUnknownAddressType
		}

	default:
		return nil, errors.New("decoded address is of unknown size")
	}
}

func newAddressScriptHashFromHash(scriptHash []byte, netID [2]byte) (*AddressScriptHash, error) {
	// Check for a valid script hash length.
	if len(scriptHash) != ripemd160.Size {
		return nil, errors.New("scriptHash must be 20 bytes")
	}

	addr := &AddressScriptHash{netID: netID}
	copy(addr.hash[:], scriptHash)
	return addr, nil
}

func newAddressPubKeyHash(pkHash []byte, netID [2]byte) (*AddressPubKeyHash, error) {
	// Check for a valid pubkey hash length.
	if len(pkHash) != ripemd160.Size {
		return nil, errors.New("pkHash must be 20 bytes")
	}

	addr := &AddressPubKeyHash{netID: netID}
	copy(addr.hash[:], pkHash)
	return addr, nil
}

// checksum: first four bytes of sha256^2
func checksum(input []byte) (cksum [4]byte) {
	h := sha256.Sum256(input)
	h2 := sha256.Sum256(h[:])
	copy(cksum[:], h2[:4])
	return
}

// CheckDecode decodes a string that was encoded with CheckEncode and verifies
// the checksum.
func CheckDecode(input string) (result []byte, version [2]byte, err error) {
	decoded := base58.Decode(input)
	if len(decoded) < 6 {
		return nil, [2]byte{0, 0}, ErrInvalidFormat
	}
	version = [2]byte{decoded[0], decoded[1]}
	var cksum [4]byte
	copy(cksum[:], decoded[len(decoded)-4:])
	if checksum(decoded[:len(decoded)-4]) != cksum {
		return nil, [2]byte{0, 0}, ErrChecksum
	}
	payload := decoded[2 : len(decoded)-4]
	result = append(result, payload...)
	return
}

// CheckEncode prepends two version bytes and appends a four byte checksum.
func CheckEncode(input []byte, version [2]byte) string {
	b := make([]byte, 0, 2+len(input)+4)
	b = append(b, version[:]...)
	b = append(b, input[:]...)
	cksum := checksum(b)
	b = append(b, cksum[:]...)
	return base58.Encode(b)
}

// ErrChecksum indicates that the checksum of a check-encoded string does not verify against
// the checksum.
var ErrChecksum = errors.New("checksum error")

// ErrInvalidFormat indicates that the check-encoded string has an invalid format.
var ErrInvalidFormat = errors.New("invalid format: version and/or checksum bytes missing")
