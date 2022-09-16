// Copyright (c) 2013-2017 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package bitcoin

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/util/base58"
	"github.com/MixinNetwork/mixin/util/bech32"
	"golang.org/x/crypto/ripemd160"
)

// UnsupportedWitnessVerError describes an error where a segwit address being
// decoded has an unsupported witness version.
type UnsupportedWitnessVerError byte

func (e UnsupportedWitnessVerError) Error() string {
	return fmt.Sprintf("unsupported witness version: %#x", byte(e))
}

// UnsupportedWitnessProgLenError describes an error where a segwit address
// being decoded has an unsupported witness program length.
type UnsupportedWitnessProgLenError int

func (e UnsupportedWitnessProgLenError) Error() string {
	return fmt.Sprintf("unsupported witness program length: %d", int(e))
}

var (
	// ErrChecksumMismatch describes an error where decoding failed due
	// to a bad checksum.
	ErrChecksumMismatch = errors.New("checksum mismatch")

	// ErrUnknownAddressType describes an error where an address can not
	// decoded as a specific address type due to the string encoding
	// beginning with an identifier byte unknown to any standard or
	// registered (via chaincfg.Register) network.
	ErrUnknownAddressType = errors.New("unknown address type")

	// ErrAddressCollision describes an error where an address can not
	// be uniquely determined as either a pay-to-pubkey-hash or
	// pay-to-script-hash address since the leading identifier is used for
	// describing both address kinds, but for different networks.  Rather
	// than assuming or defaulting to one or the other, this error is
	// returned and the caller must decide how to decode the address.
	ErrAddressCollision = errors.New("address collision")
)

// DecodeAddress decodes the string encoding of an address and returns
// the Address if addr is a valid encoding for a known address type.
//
// The bitcoin network the address is associated with is extracted if possible.
// When the address does not encode the network, such as in the case of a raw
// public key, the address will be associated with the passed defaultNet.
func DecodeCheckAddress(addr string) error {
	// Bech32 encoded segwit addresses start with a human-readable part
	// (hrp) followed by '1'. For Bitcoin mainnet the hrp is "bc", and for
	// testnet it is "tb". If the address string has a prefix that matches
	// one of the prefixes for the known networks, we try to decode it as
	// a segwit address.
	oneIndex := strings.LastIndexByte(addr, '1')
	if oneIndex > 1 {
		prefix := addr[:oneIndex+1]
		if prefix == "bc1" {
			witnessVer, witnessProg, err := decodeSegWitAddress(addr)
			if err != nil {
				return err
			}

			// We currently only support P2WPKH and P2WSH, which is
			// witness version 0 and P2TR which is witness version
			// 1.
			if witnessVer != 0 && witnessVer != 1 {
				return UnsupportedWitnessVerError(witnessVer)
			}

			if l := len(witnessProg); l != 20 && l != 32 {
				return UnsupportedWitnessProgLenError(len(witnessProg))
			}

			return nil
		}
	}

	// Serialized public keys are either 65 bytes (130 hex chars) if
	// uncompressed/hybrid or 33 bytes (66 hex chars) if compressed.
	if len(addr) == 130 || len(addr) == 66 {
		_, err := hex.DecodeString(addr)
		if err != nil {
			return err
		}
		return nil
	}

	// Switch on decoded length to determine the type.
	decoded, _, err := base58.CheckDecode(addr)
	if err != nil {
		if err == base58.ErrChecksum {
			return ErrChecksumMismatch
		}
		return errors.New("decoded address is of unknown format")
	}
	if len(decoded) != ripemd160.Size {
		return errors.New("decoded address is of unknown size")
	}
	return nil
}

// decodeSegWitAddress parses a bech32 encoded segwit address string and
// returns the witness version and witness program byte representation.
func decodeSegWitAddress(address string) (byte, []byte, error) {
	// Decode the bech32 encoded address.
	_, data, bech32version, err := bech32.DecodeGeneric(address)
	if err != nil {
		return 0, nil, err
	}

	// The first byte of the decoded address is the witness version, it must
	// exist.
	if len(data) < 1 {
		return 0, nil, fmt.Errorf("no witness version")
	}

	// ...and be <= 16.
	version := data[0]
	if version > 16 {
		return 0, nil, fmt.Errorf("invalid witness version: %v", version)
	}

	// The remaining characters of the address returned are grouped into
	// words of 5 bits. In order to restore the original witness program
	// bytes, we'll need to regroup into 8 bit words.
	regrouped, err := bech32.ConvertBits(data[1:], 5, 8, false)
	if err != nil {
		return 0, nil, err
	}

	// The regrouped data must be between 2 and 40 bytes.
	if len(regrouped) < 2 || len(regrouped) > 40 {
		return 0, nil, fmt.Errorf("invalid data length")
	}

	// For witness version 0, address MUST be exactly 20 or 32 bytes.
	if version == 0 && len(regrouped) != 20 && len(regrouped) != 32 {
		return 0, nil, fmt.Errorf("invalid data length for witness "+
			"version 0: %v", len(regrouped))
	}

	// For witness version 0, the bech32 encoding must be used.
	if version == 0 && bech32version != bech32.Version0 {
		return 0, nil, fmt.Errorf("invalid checksum expected bech32 " +
			"encoding for address with witness version 0")
	}

	// For witness version 1, the bech32m encoding must be used.
	if version == 1 && bech32version != bech32.VersionM {
		return 0, nil, fmt.Errorf("invalid checksum expected bech32m " +
			"encoding for address with witness version 1")
	}

	return version, regrouped, nil
}
