package monero

import (
	"math/big"
	"strings"
)

const BASE58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var base58Lookup = map[string]int{
	"1": 0, "2": 1, "3": 2, "4": 3, "5": 4, "6": 5, "7": 6, "8": 7,
	"9": 8, "A": 9, "B": 10, "C": 11, "D": 12, "E": 13, "F": 14, "G": 15,
	"H": 16, "J": 17, "K": 18, "L": 19, "M": 20, "N": 21, "P": 22, "Q": 23,
	"R": 24, "S": 25, "T": 26, "U": 27, "V": 28, "W": 29, "X": 30, "Y": 31,
	"Z": 32, "a": 33, "b": 34, "c": 35, "d": 36, "e": 37, "f": 38, "g": 39,
	"h": 40, "i": 41, "j": 42, "k": 43, "m": 44, "n": 45, "o": 46, "p": 47,
	"q": 48, "r": 49, "s": 50, "t": 51, "u": 52, "v": 53, "w": 54, "x": 55,
	"y": 56, "z": 57,
}
var bigBase = big.NewInt(58)

var bytesBase58LengthMapping = []int{
	0,  // 0 bytes of input, 0 byte of base58 output
	2,  // 1 byte of input, 2 bytes of base58 output
	3,  // 2 byte of input, 3 bytes of base58 output
	5,  // 3 byte of input, 5 bytes of base58 output
	6,  // 4 byte of input, 6 bytes of base58 output
	7,  // 5 byte of input, 7 bytes of base58 output
	9,  // 6 byte of input, 9 bytes of base58 output
	10, // 7 byte of input, 10 bytes of base58 output
	11, // 8 byte of input, 11 bytes of base58 output
}

func encodeChunk(raw []byte) (result string) {
	remainder := new(big.Int)
	remainder.SetBytes(raw)
	bigZero := new(big.Int)
	for remainder.Cmp(bigZero) > 0 {
		current := new(big.Int)
		remainder.DivMod(remainder, bigBase, current)
		result = string(BASE58[current.Int64()]) + result
	}

	for k, v := range bytesBase58LengthMapping {
		if k != len(raw) {
			continue
		}
		if len(result) < v {
			result = strings.Repeat("1", (v-len(result))) + result
		}
		return result
	}
	return
}

func decodeChunk(encoded string) (result []byte) {
	bigResult := big.NewInt(0)
	currentMultiplier := big.NewInt(1)
	tmp := new(big.Int)
	for i := len(encoded) - 1; i >= 0; i-- {
		if strings.IndexAny(BASE58, string(encoded[i])) < 0 {
			return
		}
		tmp.SetInt64(int64(base58Lookup[string(encoded[i])]))
		tmp.Mul(currentMultiplier, tmp)
		bigResult.Add(bigResult, tmp)
		currentMultiplier.Mul(currentMultiplier, bigBase)
	}

	for k, v := range bytesBase58LengthMapping {
		if v == len(encoded) {
			result = append([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0}, bigResult.Bytes()...)
			return result[len(result)-k:]
		}
	}
	return
}

func encodeMoneroBase58(data ...[]byte) (result string) {
	var combined []byte
	for _, item := range data {
		combined = append(combined, item...)
	}
	length := len(combined)
	rounds := length / 8
	for i := 0; i < rounds; i++ {
		result += encodeChunk(combined[i*8 : (i+1)*8])
	}
	if length%8 > 0 {
		result += encodeChunk(combined[rounds*8:])
	}
	return
}

func decodeMoneroBase58(data string) (result []byte) {
	length := len(data)
	rounds := length / 11
	for i := 0; i < rounds; i++ {
		result = append(result, decodeChunk(data[i*11:(i+1)*11])...)
	}
	if length%11 > 0 {
		result = append(result, decodeChunk(data[rounds*11:])...)
	}
	return
}
