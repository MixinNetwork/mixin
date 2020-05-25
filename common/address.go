package common

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/btcsuite/btcutil/base58"
)

const MainNetworkId = "XIN"

type Address struct {
	PrivateSpendKey crypto.PrivateKey
	PrivateViewKey  crypto.PrivateKey
	PublicSpendKey  crypto.PublicKey
	PublicViewKey   crypto.PublicKey
}

func NewAddressFromSeed(seed []byte) Address {
	hash1 := crypto.NewHash(seed)
	hash2 := crypto.NewHash(hash1[:])
	src := append(hash1[:], hash2[:]...)
	spend := crypto.NewPrivateKeyFromSeed(seed)
	view := crypto.NewPrivateKeyFromSeed(src)

	return Address{
		PrivateSpendKey: spend,
		PrivateViewKey:  view,
		PublicSpendKey:  spend.Public(),
		PublicViewKey:   view.Public(),
	}
}

func NewAddressFromString(s string) (Address, error) {
	var a Address
	if !strings.HasPrefix(s, MainNetworkId) {
		return a, errors.New("invalid address network")
	}
	data := base58.Decode(s[len(MainNetworkId):])
	if len(data) != crypto.KeySize*2+4 {
		return a, errors.New("invalid address format")
	}
	checksum := crypto.NewHash(append([]byte(MainNetworkId), data[:crypto.KeySize*2]...))
	if !bytes.Equal(checksum[:4], data[crypto.KeySize*2:]) {
		return a, errors.New("invalid address checksum")
	}
	var (
		pubSpend crypto.Key
		pubView  crypto.Key
		err      error
	)
	copy(pubSpend[:], data[:len(pubSpend)])
	copy(pubView[:], data[len(pubSpend):])
	if a.PublicSpendKey, err = pubSpend.AsPublicKey(); err != nil {
		return a, err
	}
	if a.PublicViewKey, err = pubView.AsPublicKey(); err != nil {
		return a, err
	}
	return a, nil
}

func (a Address) String() string {
	keyBts := a.PublicKeyBytes()
	data := append([]byte(MainNetworkId), keyBts...)
	checksum := crypto.NewHash(data)
	data = append(keyBts, checksum[:4]...)
	return MainNetworkId + base58.Encode(data)
}

func (a Address) PublicKeyBytes() []byte {
	var (
		pubSpend = a.PublicSpendKey.Key()
		pubView  = a.PublicViewKey.Key()
	)
	return append(pubSpend[:], pubView[:]...)
}

func (a Address) Hash() crypto.Hash {
	return crypto.NewHash(a.PublicKeyBytes())
}

func (a Address) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(a.String())), nil
}

func (a *Address) UnmarshalJSON(b []byte) error {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	m, err := NewAddressFromString(unquoted)
	if err != nil {
		return err
	}
	a.PrivateSpendKey = m.PrivateSpendKey
	a.PrivateViewKey = m.PrivateViewKey
	a.PublicSpendKey = m.PublicSpendKey
	a.PublicViewKey = m.PublicViewKey
	return nil
}
