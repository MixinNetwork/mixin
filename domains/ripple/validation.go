package ripple

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	RippleChainBase string
	RippleChainId   crypto.Hash
)

func init() {
	RippleChainBase = "23dfb5a5-5d7b-48b6-905f-3970e3176e27"
	RippleChainId = crypto.NewHash([]byte(RippleChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == RippleChainBase {
		return nil
	}
	return fmt.Errorf("invalid ripple asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid ripple address %s", address)
	}
	destinationHash, err := newHashFromString(address)
	if err != nil {
		return fmt.Errorf("invalid ripple address %s %s", address, err)
	}
	if len(destinationHash) != RIPPLE_ACCOUNT_ID_LENGTH+1 {
		return fmt.Errorf("invalid ripple address %s", address)
	}
	accountId, err := newAccountId(destinationHash[1:])
	if err != nil {
		return fmt.Errorf("invalid ripple address %s %s", address, err)
	}
	if accountId != address {
		return fmt.Errorf("invalid ripple address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if strings.TrimSpace(hash) != hash {
		return fmt.Errorf("invalid ripple transaction hash %s", hash)
	}
	if len(hash) != 64 {
		return fmt.Errorf("invalid ripple transaction hash %s", hash)
	}
	if strings.ToUpper(hash) != hash {
		return fmt.Errorf("invalid ripple transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash[:])
	if err != nil {
		return fmt.Errorf("invalid ripple transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid ripple transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case RippleChainBase:
		return RippleChainId
	default:
		panic(assetKey)
	}
}

func newHashFromString(s string) ([]byte, error) {
	decoded, err := Base58Decode(s, ALPHABET)
	if err != nil {
		return nil, err
	}
	return decoded[:len(decoded)-4], nil
}

func newAccountId(b []byte) (string, error) {
	version := RIPPLE_ACCOUNT_ID_VERSION
	if len(b) != RIPPLE_ACCOUNT_ID_LENGTH {
		return "", fmt.Errorf("Hash is wrong size, expected: %d got: %d", RIPPLE_ACCOUNT_ID_LENGTH, len(b))
	}

	h := append([]byte{byte(version)}, b...)
	return Base58Encode(h, ALPHABET), nil
}
