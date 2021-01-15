package horizen

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/ripemd160"
)

var (
	HorizenChainBase string
	HorizenChainId   crypto.Hash
)

func init() {
	HorizenChainBase = "a2c5d22b-62a2-4c13-b3f0-013290dbac60"
	HorizenChainId = crypto.NewHash([]byte(HorizenChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == HorizenChainBase {
		return nil
	}
	return fmt.Errorf("invalid horizen asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid horizen address %s", address)
	}
	horizenAddress, err := decodeAddress(address)
	if err != nil {
		return err
	}
	if horizenAddress.EncodeAddress() != address {
		return fmt.Errorf("invalid horizen address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid horizen transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid horizen transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid horizen transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid horizen transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case HorizenChainBase:
		return HorizenChainId
	default:
		panic(assetKey)
	}
}

var (
	pubKeyBase58Prefixes = [2]byte{0x20, 0x89}
	scriptBase58Prefixes = [2]byte{0x20, 0x96}
)

func encodeAddress(hash160 []byte, netID [2]byte) string {
	return CheckEncode(hash160[:ripemd160.Size], netID)
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

type AddressPubKey struct {
	pubKeyFormat btcutil.PubKeyFormat
	pubKey       *btcec.PublicKey
	pubKeyHashID [2]byte
}

func (a *AddressPubKey) addressPubKeyHash() *AddressPubKeyHash {
	addr := &AddressPubKeyHash{netID: a.pubKeyHashID}
	copy(addr.hash[:], btcutil.Hash160(a.serialize()))
	return addr
}

func (a *AddressPubKey) serialize() []byte {
	switch a.pubKeyFormat {
	default:
		fallthrough
	case btcutil.PKFUncompressed:
		return a.pubKey.SerializeUncompressed()

	case btcutil.PKFCompressed:
		return a.pubKey.SerializeCompressed()

	case btcutil.PKFHybrid:
		return a.pubKey.SerializeHybrid()
	}
}

func (a *AddressPubKey) EncodeAddress() string {
	return encodeAddress(btcutil.Hash160(a.serialize()), a.pubKeyHashID)
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

func decodeAddress(address string) (Address, error) {
	decoded, netID, err := CheckDecode(address)
	if err != nil {
		return nil, err
	}

	switch len(decoded) {
	case ripemd160.Size: // P2PKH or P2SH
		isP2PKH := netID == pubKeyBase58Prefixes
		isP2SH := netID == scriptBase58Prefixes
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

func newAddressPubKey(serializedPubKey []byte) (*AddressPubKey, error) {
	pubKey, err := btcec.ParsePubKey(serializedPubKey, btcec.S256())
	if err != nil {
		return nil, err
	}
	pkFormat := btcutil.PKFUncompressed
	switch serializedPubKey[0] {
	case 0x02, 0x03:
		pkFormat = btcutil.PKFCompressed
	case 0x06, 0x07:
		pkFormat = btcutil.PKFHybrid
	}

	return &AddressPubKey{
		pubKeyFormat: pkFormat,
		pubKey:       pubKey,
		pubKeyHashID: pubKeyBase58Prefixes,
	}, nil
}

// checksum: first four bytes of sha256^2
func checksum(input []byte) (cksum [4]byte) {
	h := sha256.Sum256(input)
	h2 := sha256.Sum256(h[:])
	copy(cksum[:], h2[:4])
	return
}

// CheckDecode decodes a string that was encoded with CheckEncode and verifies
// the checksum.
func CheckDecode(input string) (result []byte, version [2]byte, err error) {
	decoded := base58.Decode(input)
	if len(decoded) < 6 {
		return nil, [2]byte{0, 0}, ErrInvalidFormat
	}
	version = [2]byte{decoded[0], decoded[1]}
	var cksum [4]byte
	copy(cksum[:], decoded[len(decoded)-4:])
	if checksum(decoded[:len(decoded)-4]) != cksum {
		return nil, [2]byte{0, 0}, ErrChecksum
	}
	payload := decoded[2 : len(decoded)-4]
	result = append(result, payload...)
	return
}

// CheckEncode prepends two version bytes and appends a four byte checksum.
func CheckEncode(input []byte, version [2]byte) string {
	b := make([]byte, 0, 2+len(input)+4)
	b = append(b, version[:]...)
	b = append(b, input[:]...)
	cksum := checksum(b)
	b = append(b, cksum[:]...)
	return base58.Encode(b)
}

// ErrChecksum indicates that the checksum of a check-encoded string does not verify against
// the checksum.
var ErrChecksum = errors.New("checksum error")

// ErrInvalidFormat indicates that the check-encoded string has an invalid format.
var ErrInvalidFormat = errors.New("invalid format: version and/or checksum bytes missing")
