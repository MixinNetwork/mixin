package siacoin

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"golang.org/x/crypto/blake2b"
)

var (
	SiacoinChainBase string
	SiacoinChainId   crypto.Hash
)

func init() {
	SiacoinChainBase = "990c4c29-57e9-48f6-9819-7d986ea44985"
	SiacoinChainId = crypto.NewHash([]byte(SiacoinChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == SiacoinChainBase {
		return nil
	}
	return fmt.Errorf("invalid siacoin asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid siacoin address %s", address)
	}
	var uh UnlockHash
	err := uh.LoadString(address)
	if err != nil {
		return err
	}
	if uh.String() != address {
		return fmt.Errorf("invalid siacoin address %s %s", address, uh.String())
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid siacoin transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid siacoin transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid siacoin transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid siacoin transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case SiacoinChainBase:
		return SiacoinChainId
	default:
		panic(assetKey)
	}
}

const (
	HashSize               = 32
	UnlockHashChecksumSize = 6
)

type UnlockHash [38]byte

func (uh *UnlockHash) LoadString(strUH string) error {
	// Check the length of strUH.
	if len(strUH) != HashSize*2+UnlockHashChecksumSize*2 {
		return fmt.Errorf("wrong len %d", len(strUH))
	}

	// Decode the unlock hash.
	var byteUnlockHash []byte
	var checksum []byte
	_, err := fmt.Sscanf(strUH[:HashSize*2], "%x", &byteUnlockHash)
	if err != nil {
		return err
	}

	// Decode and verify the checksum.
	_, err = fmt.Sscanf(strUH[HashSize*2:], "%x", &checksum)
	if err != nil {
		return err
	}

	expectedChecksum := blake2b.Sum256(byteUnlockHash)
	if !bytes.Equal(expectedChecksum[:UnlockHashChecksumSize], checksum) {
		return fmt.Errorf("wrong checksum")
	}

	copy(uh[:], byteUnlockHash[:])
	return nil
}

func (uh UnlockHash) String() string {
	b := uh[:HashSize]
	uhChecksum := blake2b.Sum256(b)
	return fmt.Sprintf("%x%x", b, uhChecksum[:UnlockHashChecksumSize])
}
