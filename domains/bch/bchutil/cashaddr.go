package bchutil

import (
	"errors"

	"golang.org/x/crypto/ripemd160"
)

var (
	// ErrChecksumMismatch describes an error where decoding failed due
	// to a bad checksum.
	ErrChecksumMismatch = errors.New("checksum mismatch")

	Prefix = "bitcoincash"
)

type AddressType int

const (
	P2PKH AddressType = 0
	P2SH  AddressType = 1
)

type data []byte

/**
 * The cashaddr character set for encoding.
 */
const CHARSET string = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

/**
 * The cashaddr character set for decoding.
 */
var CHARSET_REV = [128]int8{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 15, -1, 10, 17, 21, 20, 26, 30, 7,
	5, -1, -1, -1, -1, -1, -1, -1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22,
	31, 27, 19, -1, 1, 0, 3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1,
	-1, -1, 29, -1, 24, 13, 25, 9, 8, 23, -1, 18, 22, 31, 27, 19, -1, 1, 0,
	3, 16, 11, 28, 12, 14, 6, 4, 2, -1, -1, -1, -1, -1,
}

/**
 * Concatenate two byte arrays.
 */
func Cat(x, y data) data {
	return append(x, y...)
}

/**
 * This function will compute what 8 5-bit values to XOR into the last 8 input
 * values, in order to make the checksum 0. These 8 values are packed together
 * in a single 40-bit integer. The higher bits correspond to earlier values.
 */
func PolyMod(v data) uint64 {
	/**
	 * The input is interpreted as a list of coefficients of a polynomial over F
	 * = GF(32), with an implicit 1 in front. If the input is [v0,v1,v2,v3,v4],
	 * that polynomial is v(x) = 1*x^5 + v0*x^4 + v1*x^3 + v2*x^2 + v3*x + v4.
	 * The implicit 1 guarantees that [v0,v1,v2,...] has a distinct checksum
	 * from [0,v0,v1,v2,...].
	 *
	 * The output is a 40-bit integer whose 5-bit groups are the coefficients of
	 * the remainder of v(x) mod g(x), where g(x) is the cashaddr generator, x^8
	 * + {19}*x^7 + {3}*x^6 + {25}*x^5 + {11}*x^4 + {25}*x^3 + {3}*x^2 + {19}*x
	 * + {1}. g(x) is chosen in such a way that the resulting code is a BCH
	 * code, guaranteeing detection of up to 4 errors within a window of 1025
	 * characters. Among the various possible BCH codes, one was selected to in
	 * fact guarantee detection of up to 5 errors within a window of 160
	 * characters and 6 erros within a window of 126 characters. In addition,
	 * the code guarantee the detection of a burst of up to 8 errors.
	 *
	 * Note that the coefficients are elements of GF(32), here represented as
	 * decimal numbers between {}. In this finite field, addition is just XOR of
	 * the corresponding numbers. For example, {27} + {13} = {27 ^ 13} = {22}.
	 * Multiplication is more complicated, and requires treating the bits of
	 * values themselves as coefficients of a polynomial over a smaller field,
	 * GF(2), and multiplying those polynomials mod a^5 + a^3 + 1. For example,
	 * {5} * {26} = (a^2 + 1) * (a^4 + a^3 + a) = (a^4 + a^3 + a) * a^2 + (a^4 +
	 * a^3 + a) = a^6 + a^5 + a^4 + a = a^3 + 1 (mod a^5 + a^3 + 1) = {9}.
	 *
	 * During the course of the loop below, `c` contains the bitpacked
	 * coefficients of the polynomial constructed from just the values of v that
	 * were processed so far, mod g(x). In the above example, `c` initially
	 * corresponds to 1 mod (x), and after processing 2 inputs of v, it
	 * corresponds to x^2 + v0*x + v1 mod g(x). As 1 mod g(x) = 1, that is the
	 * starting value for `c`.
	 */
	c := uint64(1)
	for _, d := range v {
		/**
		 * We want to update `c` to correspond to a polynomial with one extra
		 * term. If the initial value of `c` consists of the coefficients of
		 * c(x) = f(x) mod g(x), we modify it to correspond to
		 * c'(x) = (f(x) * x + d) mod g(x), where d is the next input to
		 * process.
		 *
		 * Simplifying:
		 * c'(x) = (f(x) * x + d) mod g(x)
		 *         ((f(x) mod g(x)) * x + d) mod g(x)
		 *         (c(x) * x + d) mod g(x)
		 * If c(x) = c0*x^5 + c1*x^4 + c2*x^3 + c3*x^2 + c4*x + c5, we want to
		 * compute
		 * c'(x) = (c0*x^5 + c1*x^4 + c2*x^3 + c3*x^2 + c4*x + c5) * x + d
		 *                                                             mod g(x)
		 *       = c0*x^6 + c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 + c5*x + d
		 *                                                             mod g(x)
		 *       = c0*(x^6 mod g(x)) + c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 +
		 *                                                             c5*x + d
		 * If we call (x^6 mod g(x)) = k(x), this can be written as
		 * c'(x) = (c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 + c5*x + d) + c0*k(x)
		 */

		// First, determine the value of c0:
		c0 := byte(c >> 35)

		// Then compute c1*x^5 + c2*x^4 + c3*x^3 + c4*x^2 + c5*x + d:
		c = ((c & 0x07ffffffff) << 5) ^ uint64(d)

		// Finally, for each set bit n in c0, conditionally add {2^n}k(x):
		if c0&0x01 > 0 {
			// k(x) = {19}*x^7 + {3}*x^6 + {25}*x^5 + {11}*x^4 + {25}*x^3 +
			//        {3}*x^2 + {19}*x + {1}
			c ^= 0x98f2bc8e61
		}

		if c0&0x02 > 0 {
			// {2}k(x) = {15}*x^7 + {6}*x^6 + {27}*x^5 + {22}*x^4 + {27}*x^3 +
			//           {6}*x^2 + {15}*x + {2}
			c ^= 0x79b76d99e2
		}

		if c0&0x04 > 0 {
			// {4}k(x) = {30}*x^7 + {12}*x^6 + {31}*x^5 + {5}*x^4 + {31}*x^3 +
			//           {12}*x^2 + {30}*x + {4}
			c ^= 0xf33e5fb3c4
		}

		if c0&0x08 > 0 {
			// {8}k(x) = {21}*x^7 + {24}*x^6 + {23}*x^5 + {10}*x^4 + {23}*x^3 +
			//           {24}*x^2 + {21}*x + {8}
			c ^= 0xae2eabe2a8
		}

		if c0&0x10 > 0 {
			// {16}k(x) = {3}*x^7 + {25}*x^6 + {7}*x^5 + {20}*x^4 + {7}*x^3 +
			//            {25}*x^2 + {3}*x + {16}
			c ^= 0x1e4f43e470
		}
	}

	/**
	 * PolyMod computes what value to xor into the final values to make the
	 * checksum 0. However, if we required that the checksum was 0, it would be
	 * the case that appending a 0 to a valid list of values would result in a
	 * new valid list. For that reason, cashaddr requires the resulting checksum
	 * to be 1 instead.
	 */
	return c ^ 1
}

/**
 * Convert to lower case.
 *
 * Assume the input is a character.
 */
func LowerCase(c byte) byte {
	// ASCII black magic.
	return c | 0x20
}

/**
 * Expand the address prefix for the checksum computation.
 */
func ExpandPrefix(prefix string) data {
	ret := make(data, len(prefix)+1)
	for i := 0; i < len(prefix); i++ {
		ret[i] = byte(prefix[i]) & 0x1f
	}

	ret[len(prefix)] = 0
	return ret
}

/**
 * Verify a checksum.
 */
func VerifyChecksum(prefix string, payload data) bool {
	return PolyMod(Cat(ExpandPrefix(prefix), payload)) == 0
}

/**
 * Create a checksum.
 */
func CreateChecksum(prefix string, payload data) data {
	enc := Cat(ExpandPrefix(prefix), payload)
	// Append 8 zeroes.
	enc = Cat(enc, data{0, 0, 0, 0, 0, 0, 0, 0})
	// Determine what to XOR into those 8 zeroes.
	mod := PolyMod(enc)
	ret := make(data, 8)
	for i := 0; i < 8; i++ {
		// Convert the 5-bit groups in mod to checksum values.
		ret[i] = byte((mod >> uint(5*(7-i))) & 0x1f)
	}
	return ret
}

/**
 * Encode a cashaddr string.
 */
func Encode(prefix string, payload data) string {
	checksum := CreateChecksum(prefix, payload)
	combined := Cat(payload, checksum)
	ret := ""

	for _, c := range combined {
		ret += string(CHARSET[c])
	}

	return ret
}

/**
 * Decode a cashaddr string.
 */
func DecodeCashAddress(str string) (string, data, error) {
	// Go over the string and do some sanity checks.
	lower, upper := false, false
	prefixSize := 0
	for i := 0; i < len(str); i++ {
		c := byte(str[i])
		if c >= 'a' && c <= 'z' {
			lower = true
			continue
		}

		if c >= 'A' && c <= 'Z' {
			upper = true
			continue
		}

		if c >= '0' && c <= '9' {
			// We cannot have numbers in the prefix.
			if prefixSize == 0 {
				return "", data{}, errors.New("Addresses cannot have numbers in the prefix")
			}

			continue
		}

		if c == ':' {
			// The separator must not be the first character, and there must not
			// be 2 separators.
			if i == 0 || prefixSize != 0 {
				return "", data{}, errors.New("The separator must not be the first character")
			}

			prefixSize = i
			continue
		}

		// We have an unexpected character.
		return "", data{}, errors.New("Unexpected character")
	}

	// We must have a prefix and a data part and we can't have both uppercase
	// and lowercase.
	if prefixSize == 0 {
		return "", data{}, errors.New("Address must have a prefix")
	}

	if upper && lower {
		return "", data{}, errors.New("Addresses cannot use both upper and lower case characters")
	}

	// Get the prefix.
	var prefix string
	for i := 0; i < prefixSize; i++ {
		prefix += string(LowerCase(str[i]))
	}

	// Decode values.
	valuesSize := len(str) - 1 - prefixSize
	values := make(data, valuesSize)
	for i := 0; i < valuesSize; i++ {
		c := byte(str[i+prefixSize+1])
		// We have an invalid char in there.
		if c > 127 || CHARSET_REV[c] == -1 {
			return "", data{}, errors.New("Invalid character")
		}

		values[i] = byte(CHARSET_REV[c])
	}

	// Verify the checksum.
	if !VerifyChecksum(prefix, values) {
		return "", data{}, ErrChecksumMismatch
	}

	return prefix, values[:len(values)-8], nil
}

func CheckEncodeCashAddress(input []byte, prefix string, t AddressType) string {
	k, err := packAddressData(t, input)
	if err != nil {
		return ""
	}
	return Encode(prefix, k)
}

// CheckDecode decodes a string that was encoded with CheckEncode and verifies the checksum.
func CheckDecodeCashAddress(input string) (result []byte, prefix string, t AddressType, err error) {
	prefix, data, err := DecodeCashAddress(input)
	if err != nil {
		return data, prefix, P2PKH, err
	}
	data, err = convertBits(data, 5, 8, false)
	if err != nil {
		return data, prefix, P2PKH, err
	}
	if len(data) != 21 {
		return data, prefix, P2PKH, errors.New("Incorrect data length")
	}
	switch data[0] {
	case 0x00:
		t = P2PKH
	case 0x08:
		t = P2SH
	}
	return data[1:21], prefix, t, nil
}

// encodeAddress returns a human-readable payment address given a ripemd160 hash
// and prefix which encodes the bitcoin cash network and address type.  It is used
// in both pay-to-pubkey-hash (P2PKH) and pay-to-script-hash (P2SH) address
// encoding.
func encodeCashAddress(hash160 []byte, prefix string, t AddressType) string {
	return CheckEncodeCashAddress(hash160[:ripemd160.Size], prefix, t)
}

// DecodeAddress decodes the string encoding of an address and returns
// the Address if addr is a valid encoding for a known address type.
//
// The bitcoin cash network the address is associated with is extracted if possible.
func VerifyAddress(addr string) error {
	// Add prefix if it does not exist
	if len(addr) >= len(Prefix)+1 && addr[:len(Prefix)+1] != Prefix+":" {
		addr = Prefix + ":" + addr
	}

	// Switch on decoded length to determine the type.
	decoded, _, _, err := CheckDecodeCashAddress(addr)
	if err != nil {
		if err == ErrChecksumMismatch {
			return ErrChecksumMismatch
		}
		return errors.New("decoded address is of unknown format")
	}
	if len(decoded) != ripemd160.Size {
		return errors.New("decoded address is of unknown size")
	}
	return nil
}

// Base32 conversion contains some licensed code

// https://github.com/sipa/bech32/blob/master/ref/go/src/bech32/bech32.go
// Copyright (c) 2017 Takatoshi Nakagawa
// MIT License

func convertBits(data data, fromBits uint, tobits uint, pad bool) (data, error) {
	// General power-of-2 base conversion.
	var uintArr []uint
	for _, i := range data {
		uintArr = append(uintArr, uint(i))
	}
	acc := uint(0)
	bits := uint(0)
	var ret []uint
	maxv := uint((1 << tobits) - 1)
	maxAcc := uint((1 << (fromBits + tobits - 1)) - 1)
	for _, value := range uintArr {
		acc = ((acc << fromBits) | value) & maxAcc
		bits += fromBits
		for bits >= tobits {
			bits -= tobits
			ret = append(ret, (acc>>bits)&maxv)
		}
	}
	if pad {
		if bits > 0 {
			ret = append(ret, (acc<<(tobits-bits))&maxv)
		}
	} else if bits >= fromBits || ((acc<<(tobits-bits))&maxv) != 0 {
		return []byte{}, errors.New("encoding padding error")
	}
	var dataArr []byte
	for _, i := range ret {
		dataArr = append(dataArr, byte(i))
	}
	return dataArr, nil
}

func packAddressData(addrType AddressType, addrHash data) (data, error) {
	// Pack addr data with version byte.
	if addrType != P2PKH && addrType != P2SH {
		return data{}, errors.New("invalid addrtype")
	}
	versionByte := uint(addrType) << 3
	encodedSize := (uint(len(addrHash)) - 20) / 4
	if (len(addrHash)-20)%4 != 0 {
		return data{}, errors.New("invalid addrhash size")
	}
	if encodedSize < 0 || encodedSize > 8 {
		return data{}, errors.New("encoded size out of valid range")
	}
	versionByte |= encodedSize
	var addrHashUint data
	for _, e := range addrHash {
		addrHashUint = append(addrHashUint, byte(e))
	}
	data := append([]byte{byte(versionByte)}, addrHashUint...)
	packedData, err := convertBits(data, 8, 5, true)
	if err != nil {
		return []byte{}, err
	}
	return packedData, nil
}
