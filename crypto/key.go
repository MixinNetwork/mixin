package crypto

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"strconv"
)

type Key [32]byte

func NewKeyFromSeed(seed []byte) PrivateKey {
	return keyFactory.NewPrivateKeyFromSeedPanic(seed)
}

func KeyFromString(s string) (Key, error) {
	var key Key
	b, err := hex.DecodeString(s)
	if err != nil {
		return key, err
	}
	if len(b) != len(key) {
		return key, fmt.Errorf("invalid key size %d", len(b))
	}
	copy(key[:], b)
	return key, nil
}

func (k Key) HasValue() bool {
	empty := Key{}
	return bytes.Compare(k[:], empty[:]) != 0
}

func (k Key) String() string {
	return hex.EncodeToString(k[:])
}

func (k Key) AsPrivateKey() (PrivateKey, error) {
	return keyFactory.PrivateKeyFromKey(k)
}

func (k Key) AsPrivateKeyPanic() PrivateKey {
	key, err := keyFactory.PrivateKeyFromKey(k)
	if err != nil {
		panic(err)
	}
	return key
}

func (k Key) AsPublicKey() (PublicKey, error) {
	return keyFactory.PublicKeyFromKey(k)
}

func (k Key) AsPublicKeyPanic() PublicKey {
	key, err := keyFactory.PublicKeyFromKey(k)
	if err != nil {
		panic(err)
	}
	return key
}

func (k Key) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(k.String())), nil
}

func (k *Key) UnmarshalJSON(b []byte) error {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	data, err := hex.DecodeString(string(unquoted))
	if err != nil {
		return err
	}
	if len(data) != len(k) {
		return fmt.Errorf("invalid key length %d", len(data))
	}
	copy(k[:], data)
	return nil
}

// Scan implements the sql.Scanner interface for database deserialization.
func (k *Key) Scan(value interface{}) (err error) {
	var s string
	switch v := value.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	}
	*k, err = KeyFromString(s)
	return
}

// Value implements the driver.Valuer interface for database serialization.
func (k Key) Value() (driver.Value, error) {
	return k.String(), nil
}
