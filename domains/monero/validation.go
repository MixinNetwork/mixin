package monero

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/paxosglobal/moneroutil"
	"golang.org/x/crypto/sha3"
)

var (
	MoneroChainBase string
	MoneroChainId   crypto.Hash
)

func init() {
	MoneroChainBase = "05c5ac01-31f9-4a69-aa8a-ab796de1d041"
	MoneroChainId = crypto.NewHash([]byte(MoneroChainBase))
}

func VerifyAssetKey(assetKey string) error {
	if assetKey == MoneroChainBase {
		return nil
	}
	return fmt.Errorf("invalid monero asset key %s", assetKey)
}

func VerifyAddress(address string) error {
	if strings.TrimSpace(address) != address {
		return fmt.Errorf("invalid monero address %s", address)
	}
	addr := moneroutil.DecodeMoneroBase58(address)
	if len(addr) != 69 {
		return fmt.Errorf("invalid monero address %s", address)
	}
	checksum := Keccak256(addr[:len(addr)-4])[:4]
	if bytes.Compare(checksum, addr[len(addr)-4:]) != 0 {
		return fmt.Errorf("invalid monero address %s", address)
	}
	if addr[0] != 18 && addr[0] != 42 {
		return fmt.Errorf("invalid monero address %s", address)
	}
	checksum = Keccak256([]byte{addr[0]}, addr[1:33], addr[33:65])[:4]
	result := moneroutil.EncodeMoneroBase58([]byte{addr[0]}, addr[1:33], addr[33:65], checksum[:])
	if result != address {
		return fmt.Errorf("invalid monero address %s", address)
	}
	return nil
}

func VerifyTransactionHash(hash string) error {
	if len(hash) != 64 {
		return fmt.Errorf("invalid monero transaction hash %s", hash)
	}
	if strings.ToLower(hash) != hash {
		return fmt.Errorf("invalid monero transaction hash %s", hash)
	}
	h, err := hex.DecodeString(hash)
	if err != nil {
		return fmt.Errorf("invalid monero transaction hash %s %s", hash, err.Error())
	}
	if len(h) != 32 {
		return fmt.Errorf("invalid monero transaction hash %s", hash)
	}
	return nil
}

func GenerateAssetId(assetKey string) crypto.Hash {
	switch assetKey {
	case MoneroChainBase:
		return MoneroChainId
	default:
		panic(assetKey)
	}
}

type KeccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

func NewKeccakState() KeccakState {
	return sha3.NewLegacyKeccak256().(KeccakState)
}

func Keccak256(data ...[]byte) []byte {
	b := make([]byte, 32)
	d := NewKeccakState()
	for _, b := range data {
		d.Write(b)
	}
	d.Read(b)
	return b
}
