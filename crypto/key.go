package crypto

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
)

var (
	emptyKey = Key{}
)

func NewPrivateKeyFromReader(randReader io.Reader) PrivateKey {
	var (
		seed = make([]byte, 64)
		s    = 0
	)

	for s < len(seed) {
		n, err := randReader.Read(seed[s:])
		if err != nil {
			return nil
		}
		s += n
	}
	return keyFactory.NewPrivateKeyFromSeedOrPanic(seed)
}

func NewPrivateKeyFromSeed(seed []byte) PrivateKey {
	return keyFactory.NewPrivateKeyFromSeedOrPanic(seed)
}

func PrivateKeyFromString(s string) (PrivateKey, error) {
	key, err := KeyFromString(s)
	if err != nil {
		return nil, err
	}
	return key.AsPrivateKey()
}

func PublicKeyFromString(s string) (PublicKey, error) {
	key, err := KeyFromString(s)
	if err != nil {
		return nil, err
	}
	return key.AsPublicKey()
}

func KeyFromString(s string) (*Key, error) {
	var key Key
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != len(key) {
		return nil, fmt.Errorf("invalid key size %d", len(b))
	}
	copy(key[:], b)
	return &key, nil
}

func (k Key) AsPrivateKey() (PrivateKey, error) {
	return keyFactory.PrivateKeyFromKey(k)
}

func (k Key) AsPublicKey() (PublicKey, error) {
	return keyFactory.PublicKeyFromKey(k)
}

func (k Key) HasValue() bool {
	return bytes.Compare(k[:], emptyKey[:]) != 0
}

func (k Key) String() string {
	return hex.EncodeToString(k[:])
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
func (k *Key) Scan(value interface{}) error {
	var s string
	switch v := value.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	}
	key, err := KeyFromString(s)
	if err != nil {
		return err
	}
	copy(k[:], key[:])
	return nil
}

// Value implements the driver.Valuer interface for database serialization.
func (k Key) Value() (driver.Value, error) {
	return k.String(), nil
}
