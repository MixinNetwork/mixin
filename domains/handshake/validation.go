package handshake

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcutil/bech32"
)

var (
	HandshakenChainBase string
	HandshakenChainId   crypto.Hash
	Bech32HRPSegwit     = "hs"
)

func init() {
	HandshakenChainBase = "13036886-6b83-4ced-8d44-9f69151587bf"
	HandshakenChainId = crypto.NewHash([]byte(HandshakenChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == HandshakenChainBase {
		return nil
	}
	return fmt.Errorf("invalid handshake asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid handshake address %s", address)
	}
	err := validateAddress(address)
	if err != nil {
		return fmt.Errorf("invalid handshake address %s %s", address, err)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid handshake transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid handshake transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid handshake transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid handshake transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case HandshakenChainBase:
		return HandshakenChainId
	default:
		panic(assetKey)
	}
}

func validateAddress(addr string) error {
	witnessVer, witnessProg, err := decodeSegWitAddress(addr)
	if err != nil {
		return err
	}

	if witnessVer != 0 {
		return fmt.Errorf("UnsupportedWitnessVerError(%d)", witnessVer)
	}

	switch len(witnessProg) {
	case 20, 32:
		re, err := encodeSegWitAddress(Bech32HRPSegwit, 0x00, witnessProg)
		if err != nil {
			return err
		}
		if re != addr {
			return fmt.Errorf("Malformed witness address %s %s", addr, re)
		}
	default:
		return fmt.Errorf("UnsupportedWitnessProgLenError(%d)", len(witnessProg))
	}

	return nil
}

func decodeSegWitAddress(address string) (byte, []byte, error) {
	// Decode the bech32 encoded address.
	_, data, err := bech32.Decode(address)
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

	return version, regrouped, nil
}

func encodeSegWitAddress(hrp string, witnessVersion byte, witnessProgram []byte) (string, error) {
	// Group the address bytes into 5 bit groups, as this is what is used to
	// encode each character in the address string.
	converted, err := bech32.ConvertBits(witnessProgram, 8, 5, true)
	if err != nil {
		return "", err
	}

	// Concatenate the witness version and program, and encode the resulting
	// bytes using bech32 encoding.
	combined := make([]byte, len(converted)+1)
	combined[0] = witnessVersion
	copy(combined[1:], converted)
	bech, err := bech32.Encode(hrp, combined)
	if err != nil {
		return "", err
	}

	return bech, nil
}
