package avalanche

import (
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/util/base58"
	"github.com/MixinNetwork/mixin/util/bech32"
)

var (
	AvalancheAssetKey  string
	AvalancheChainBase string
	AvalancheChainId   crypto.Hash
)

func init() {
	AvalancheAssetKey = "FvwEAhmxKfeiG8SnEvq42hc6whRyY3EFYAvebMqDNDGCgxN5Z"
	AvalancheChainBase = "cbc77539-0a20-4666-8c8a-4ded62b36f0a"
	AvalancheChainId = crypto.NewHash([]byte(AvalancheChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == AvalancheAssetKey {
		return nil
	}
	return fmt.Errorf("invalid avalanche asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid avalanche address %s", address)
	}
	chainId, hrp, addr, err := parseAddress(address)
	if err != nil {
		return err
	}
	if chainId != "X" {
		return fmt.Errorf("bad chainId %s", chainId)
	}
	if hrp != MainnetHRP {
		return fmt.Errorf("bad hrp %s", hrp)
	}
	if len(addr) != 20 {
		return fmt.Errorf("bad addr length %d", len(addr))
	}
	formated, err := formatAddress(chainId, hrp, addr)
	if err != nil {
		return err
	}
	if formated != address {
		return fmt.Errorf("avalanche address mismatch %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	decodedBytes := base58.Decode(hash)
	if len(decodedBytes) != 36 {
		return fmt.Errorf("invalid avalanche transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case AvalancheAssetKey:
		return AvalancheChainId
	default:
		panic(assetKey)
	}
}

const (
	addressSep = "-"
	MainnetHRP = "avax"
)

// ParseAddress takes in an address string and splits returns the corresponding
// parts. This returns the chain ID alias, bech32 HRP, address bytes, and an
// error if it occurs.
func parseAddress(addrStr string) (string, string, []byte, error) {
	addressParts := strings.SplitN(addrStr, addressSep, 2)
	if len(addressParts) < 2 {
		return "", "", nil, fmt.Errorf("no separator found in address")
	}
	chainID := addressParts[0]
	rawAddr := addressParts[1]

	hrp, addr, err := parseBech32(rawAddr)
	return chainID, hrp, addr, err
}

// ParseBech32 takes a bech32 address as input and returns the HRP and data
// section of a bech32 address
func parseBech32(addrStr string) (string, []byte, error) {
	rawHRP, decoded, err := bech32.Decode(addrStr)
	if err != nil {
		return "", nil, err
	}
	addrBytes, err := bech32.ConvertBits(decoded, 5, 8, true)
	if err != nil {
		return "", nil, fmt.Errorf("unable to convert address from 5-bit to 8-bit formatting")
	}
	return rawHRP, addrBytes, nil
}

func formatAddress(
	chainIDAlias string,
	hrp string,
	addr []byte,
) (string, error) {
	addrStr, err := formatBech32(hrp, addr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%s%s", chainIDAlias, addressSep, addrStr), nil
}

func formatBech32(hrp string, payload []byte) (string, error) {
	fiveBits, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("unable to convert address from 8-bit to 5-bit formatting")
	}
	addr, err := bech32.Encode(hrp, fiveBits)
	if err != nil {
		return "", err
	}
	return addr, nil
}
