// Copyright (c) 2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package decred

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
)

// AddressParamsV0 defines an interface that is used to provide the parameters
// required when encoding and decoding addresses for version 0 scripts.  These
// values are typically well-defined and unique per network.
type AddressParamsV0 interface {
	// AddrIDPubKeyV0 returns the magic prefix bytes for version 0 pay-to-pubkey
	// addresses.
	AddrIDPubKeyV0() [2]byte

	// AddrIDPubKeyHashECDSAV0 returns the magic prefix bytes for version 0
	// pay-to-pubkey-hash addresses where the underlying pubkey is secp256k1 and
	// the signature algorithm is ECDSA.
	AddrIDPubKeyHashECDSAV0() [2]byte

	// AddrIDPubKeyHashEd25519V0 returns the magic prefix bytes for version 0
	// pay-to-pubkey-hash addresses where the underlying pubkey and signature
	// algorithm are Ed25519.
	AddrIDPubKeyHashEd25519V0() [2]byte

	// AddrIDPubKeyHashSchnorrV0 returns the magic prefix bytes for version 0
	// pay-to-pubkey-hash addresses where the underlying pubkey is secp256k1 and
	// the signature algorithm is Schnorr.
	AddrIDPubKeyHashSchnorrV0() [2]byte

	// AddrIDScriptHashV0 returns the magic prefix bytes for version 0
	// pay-to-script-hash addresses.
	AddrIDScriptHashV0() [2]byte
}

// DecodeAddressV0 decodes the string encoding of an address and returns the
// relevant Address if it is a valid encoding for a known version 0 address type
// and is for the network identified by the provided parameters.
func DecodeAddressV0(addr string, params AddressParamsV0) error {
	// Attempt to decode the address and address type.
	_, addrID, err := CheckDecode(addr)
	if err != nil {
		kind := ErrMalformedAddress
		str := fmt.Sprintf("failed to decoded address %q: %v", addr, err)
		return makeError(kind, str)
	}

	// Decode the address according to the address type.
	switch addrID {
	case params.AddrIDScriptHashV0():
	case params.AddrIDPubKeyHashECDSAV0():
	case params.AddrIDPubKeyHashSchnorrV0():
	case params.AddrIDPubKeyHashEd25519V0():
	case params.AddrIDPubKeyV0():
	default:
		str := fmt.Sprintf("address %q is not a supported type", addr)
		return makeError(ErrUnsupportedAddress, str)
	}

	return nil
}

// ErrChecksum indicates that the checksum of a check-encoded string does not verify against
// the checksum.
var ErrChecksum = errors.New("checksum error")

// ErrInvalidFormat indicates that the check-encoded string has an invalid format.
var ErrInvalidFormat = errors.New("invalid format: version and/or checksum bytes missing")

// checksum returns the first four bytes of BLAKE256(BLAKE256(input)).
func checksum(input []byte) [4]byte {
	var calculatedChecksum [4]byte
	intermediateHash := Sum256(input)
	finalHash := Sum256(intermediateHash[:])
	copy(calculatedChecksum[:], finalHash[:])
	return calculatedChecksum
}

// CheckDecode decodes a string that was encoded with CheckEncode and verifies
// the checksum.
func CheckDecode(input string) ([]byte, [2]byte, error) {
	decoded := base58.Decode(input)
	if len(decoded) < 6 {
		return nil, [2]byte{0, 0}, ErrInvalidFormat
	}
	version := [2]byte{decoded[0], decoded[1]}
	dataLen := len(decoded) - 4
	decodedChecksum := decoded[dataLen:]
	calculatedChecksum := checksum(decoded[:dataLen])
	if !bytes.Equal(decodedChecksum, calculatedChecksum[:]) {
		return nil, [2]byte{0, 0}, ErrChecksum
	}
	payload := decoded[2:dataLen]
	return payload, version, nil
}
