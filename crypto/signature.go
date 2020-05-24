package crypto

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

type Signature [64]byte

func (s *Signature) WithCommitment(commitment *Commitment) {
	keyFactory.UpdateSignatureCommitment(s, commitment)
}

func (s Signature) String() string {
	return hex.EncodeToString(s[:])
}

func (s Signature) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(s.String())), nil
}

func (s *Signature) UnmarshalJSON(b []byte) error {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	data, err := hex.DecodeString(string(unquoted))
	if err != nil {
		return err
	}
	if len(data) != len(s) {
		return fmt.Errorf("invalid signature length %d", len(data))
	}
	copy(s[:], data)
	return nil
}
