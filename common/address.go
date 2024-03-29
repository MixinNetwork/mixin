package common

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/util/base58"
)

const MainAddressPrefix = "XIN"

type Address struct {
	PrivateSpendKey crypto.Key
	PrivateViewKey  crypto.Key
	PublicSpendKey  crypto.Key
	PublicViewKey   crypto.Key
}

func NewAddressFromSeed(seed []byte) Address {
	hash1 := crypto.Sha256Hash(seed)
	hash2 := crypto.Sha256Hash(hash1[:])
	src := append(hash1[:], hash2[:]...)
	spend := crypto.NewKeyFromSeed(seed)
	view := crypto.NewKeyFromSeed(src)

	return Address{
		PrivateSpendKey: spend,
		PrivateViewKey:  view,
		PublicSpendKey:  spend.Public(),
		PublicViewKey:   view.Public(),
	}
}

func NewAddressFromString(s string) (Address, error) {
	var a Address
	if !strings.HasPrefix(s, MainAddressPrefix) {
		return a, errors.New("invalid address network")
	}
	data := base58.Decode(s[len(MainAddressPrefix):])
	if len(data) != 68 {
		return a, errors.New("invalid address format")
	}
	checksum := crypto.Sha256Hash(append([]byte(MainAddressPrefix), data[:64]...))
	if !bytes.Equal(checksum[:4], data[64:]) {
		return a, errors.New("invalid address checksum")
	}
	copy(a.PublicSpendKey[:], data[:32])
	if !a.PublicSpendKey.CheckKey() {
		return a, errors.New("invalid address public spend key")
	}
	copy(a.PublicViewKey[:], data[32:])
	if !a.PublicViewKey.CheckKey() {
		return a, errors.New("invalid address public view key")
	}
	return a, nil
}

func (a Address) String() string {
	data := append([]byte(MainAddressPrefix), a.PublicSpendKey[:]...)
	data = append(data, a.PublicViewKey[:]...)
	checksum := crypto.Sha256Hash(data)
	data = append(a.PublicSpendKey[:], a.PublicViewKey[:]...)
	data = append(data, checksum[:4]...)
	return MainAddressPrefix + base58.Encode(data)
}

func (a Address) Hash() crypto.Hash {
	return crypto.Sha256Hash(append(a.PublicSpendKey[:], a.PublicViewKey[:]...))
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
