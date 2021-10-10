package decred

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
)

var (
	DecredChainBase string
	DecredChainId   crypto.Hash
)

func init() {
	DecredChainBase = "8f5caf2a-283d-4c85-832a-91e83bbf290b"
	DecredChainId = crypto.NewHash([]byte(DecredChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == DecredChainBase {
		return nil
	}
	return fmt.Errorf("invalid decred asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid decred address %s", address)
	}
	err := DecodeAddressV0(address, mockMainNetParams())
	if err != nil {
		return fmt.Errorf("invalid decred address %s %s", address, err)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid decred transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid decred transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid decred transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid decred transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case DecredChainBase:
		return DecredChainId
	default:
		panic(assetKey)
	}
}

// mockAddrParams implements the AddressParams interface and is used throughout
// the tests to mock multiple networks.
type mockAddrParams struct {
	pubKeyID     [2]byte
	pkhEcdsaID   [2]byte
	pkhEd25519ID [2]byte
	pkhSchnorrID [2]byte
	scriptHashID [2]byte
	privKeyID    [2]byte
}

// AddrIDPubKeyV0 returns the magic prefix bytes associated with the mock params
// for version 0 pay-to-pubkey addresses.
//
// This is part of the AddressParams interface.
func (p *mockAddrParams) AddrIDPubKeyV0() [2]byte {
	return p.pubKeyID
}

// AddrIDPubKeyHashECDSAV0 returns the magic prefix bytes associated with the
// mock params for version 0 pay-to-pubkey-hash addresses where the underlying
// pubkey is secp256k1 and the signature algorithm is ECDSA.
//
// This is part of the AddressParams interface.
func (p *mockAddrParams) AddrIDPubKeyHashECDSAV0() [2]byte {
	return p.pkhEcdsaID
}

// AddrIDPubKeyHashEd25519V0 returns the magic prefix bytes associated with the
// mock params for version 0 pay-to-pubkey-hash addresses where the underlying
// pubkey and signature algorithm are Ed25519.
//
// This is part of the AddressParams interface.
func (p *mockAddrParams) AddrIDPubKeyHashEd25519V0() [2]byte {
	return p.pkhEd25519ID
}

// AddrIDPubKeyHashSchnorrV0 returns the magic prefix bytes associated with the
// mock params for version 0 pay-to-pubkey-hash addresses where the underlying
// pubkey is secp256k1 and the signature algorithm is Schnorr.
//
// This is part of the AddressParams interface.
func (p *mockAddrParams) AddrIDPubKeyHashSchnorrV0() [2]byte {
	return p.pkhSchnorrID
}

// AddrIDScriptHashV0 returns the magic prefix bytes associated with the mock
// params for version 0 pay-to-script-hash addresses.
//
// This is part of the AddressParams interface.
func (p *mockAddrParams) AddrIDScriptHashV0() [2]byte {
	return p.scriptHashID
}

// mockMainNetParams returns mock mainnet address parameters to use throughout
// the tests.  They match the Decred mainnet params as of the time this comment
// was written.
func mockMainNetParams() *mockAddrParams {
	return &mockAddrParams{
		pubKeyID:     [2]byte{0x13, 0x86}, // starts with Dk
		pkhEcdsaID:   [2]byte{0x07, 0x3f}, // starts with Ds
		pkhEd25519ID: [2]byte{0x07, 0x1f}, // starts with De
		pkhSchnorrID: [2]byte{0x07, 0x01}, // starts with DS
		scriptHashID: [2]byte{0x07, 0x1a}, // starts with Dc
		privKeyID:    [2]byte{0x22, 0xde}, // starts with Pm
	}
}
