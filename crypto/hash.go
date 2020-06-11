package crypto

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"strconv"
)

type Hash [32]byte

func NewHash(data []byte) Hash {
	return Hash(hashFunc(data))
}

func HashFromString(src string) (Hash, error) {
	var hash Hash
	data, err := hex.DecodeString(src)
	if err != nil {
		return hash, err
	}
	if len(data) != len(hash) {
		return hash, fmt.Errorf("invalid hash length %d", len(data))
	}
	copy(hash[:], data)
	return hash, nil
}

func (h Hash) HasValue() bool {
	zero := Hash{}
	return bytes.Compare(h[:], zero[:]) != 0
}

func (h Hash) ForNetwork(net Hash) Hash {
	return NewHash(append(net[:], h[:]...))
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

func (h Hash) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(h.String())), nil
}

func (h *Hash) UnmarshalJSON(b []byte) error {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	data, err := hex.DecodeString(string(unquoted))
	if err != nil {
		return err
	}
	if len(data) != len(h) {
		return fmt.Errorf("invalid hash length %d", len(data))
	}
	copy(h[:], data)
	return nil
}

// Scan implements the sql.Scanner interface for database deserialization.
func (h *Hash) Scan(value interface{}) (err error) {
	var s string
	switch v := value.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	}
	*h, err = HashFromString(s)
	return
}

// Value implements the driver.Valuer interface for database serialization.
func (h Hash) Value() (driver.Value, error) {
	return h.String(), nil
}
