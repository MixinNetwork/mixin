package zcash

import (
	"errors"

	"github.com/MixinNetwork/mixin/domains/tezos"
	"github.com/btcsuite/btcutil"
	"golang.org/x/crypto/ripemd160"
)

type Params struct {
	PubKeyBase58Prefixes [2]byte
	ScriptBase58Prefixes [2]byte
}

func encodeAddress(hash160 []byte, netID [2]byte) string {
	return tezos.CheckEncode(hash160[:ripemd160.Size], netID[:])
}

type Address interface {
	EncodeAddress() string
}

type AddressPubKeyHash struct {
	hash  [ripemd160.Size]byte
	netID [2]byte
}

func (a *AddressPubKeyHash) EncodeAddress() string {
	return encodeAddress(a.hash[:], a.netID)
}

type AddressScriptHash struct {
	hash  [ripemd160.Size]byte
	netID [2]byte
}

// EncodeAddress returns the string encoding of a pay-to-script-hash
// address.  Part of the Address interface.
func (a *AddressScriptHash) EncodeAddress() string {
	return encodeAddress(a.hash[:], a.netID)
}

func DecodeAddress(address string, params *Params) (Address, error) {
	decoded, netIB, err := tezos.CheckDecode(address, 2)
	if err != nil {
		return nil, err
	}
	var netID [2]byte
	copy(netID[:], netIB)

	switch len(decoded) {
	case ripemd160.Size: // P2PKH or P2SH
		isP2PKH := netID == params.PubKeyBase58Prefixes
		isP2SH := netID == params.ScriptBase58Prefixes
		switch hash160 := decoded; {
		case isP2PKH && isP2SH:
			return nil, btcutil.ErrAddressCollision
		case isP2PKH:
			return newAddressPubKeyHash(hash160, netID)
		case isP2SH:
			return newAddressScriptHashFromHash(hash160, netID)
		default:
			return nil, btcutil.ErrUnknownAddressType
		}

	default:
		return nil, errors.New("decoded address is of unknown size")
	}
}

func newAddressScriptHashFromHash(scriptHash []byte, netID [2]byte) (*AddressScriptHash, error) {
	// Check for a valid script hash length.
	if len(scriptHash) != ripemd160.Size {
		return nil, errors.New("scriptHash must be 20 bytes")
	}

	addr := &AddressScriptHash{netID: netID}
	copy(addr.hash[:], scriptHash)
	return addr, nil
}

func newAddressPubKeyHash(pkHash []byte, netID [2]byte) (*AddressPubKeyHash, error) {
	// Check for a valid pubkey hash length.
	if len(pkHash) != ripemd160.Size {
		return nil, errors.New("pkHash must be 20 bytes")
	}

	addr := &AddressPubKeyHash{netID: netID}
	copy(addr.hash[:], pkHash)
	return addr, nil
}
