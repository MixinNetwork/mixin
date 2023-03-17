package stellar

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"

	"filippo.io/edwards25519"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/pkg/errors"
)

var (
	StellarChainBase string
	StellarChainId   crypto.Hash
)

func init() {
	StellarChainBase = "56e63c06-b506-4ec5-885a-4a5ac17b83c1"
	StellarChainId = crypto.NewHash([]byte(StellarChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == StellarChainBase {
		return nil
	}
	return fmt.Errorf("invalid stellar asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid stellar address %s", address)
	}
	if strings.ToUpper(address) != address {
		return fmt.Errorf("invalid stellar address %s", address)
	}
	if err := isValidEd25519PublicKey(address); err != nil {
		return isValidMuxedAccountEd25519PublicKey(address)
	}
	return nil
}

func isValidEd25519PublicKey(address string) error {
	payload, err := Decode(VersionByteAccountID, address)
	if err != nil {
		return fmt.Errorf("invalid stellar address %s %s", address, err)
	}
	if len(payload) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid stellar address length %s", address)
	}
	_, err = edwards25519.NewIdentityPoint().SetBytes(payload)
	if err != nil {
		return fmt.Errorf("invalid stellar address %s", address)
	}
	return nil
}

func isValidMuxedAccountEd25519PublicKey(s string) error {
	payload, err := Decode(VersionByteMuxedAccount, s)
	if err != nil {
		return fmt.Errorf("invalid stellar address %s %s", s, err)
	}
	if len(payload) != 40 {
		return errors.Errorf("invalid binary length: %d", len(payload))
	}

	var muxed [32]byte
	copy(muxed[:], payload[:32])

	_, err = edwards25519.NewIdentityPoint().SetBytes(muxed[:])
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("invalid stellar ed address %s", s)
	}
	return err
}

func VerifyTransactionHash(hash string) error {
	if strings.TrimSpace(hash) != hash {
		return fmt.Errorf("invalid stellar transaction hash %s", hash)
	}
	if len(hash) != 64 {
		return fmt.Errorf("invalid stellar transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid stellar transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash[:])
	if err != nil {
		return fmt.Errorf("invalid stellar transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid stellar transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case StellarChainBase:
		return StellarChainId
	default:
		panic(assetKey)
	}
}
