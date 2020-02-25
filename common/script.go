package common

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

const (
	Operator0   = 0x00
	Operator64  = 0x40
	OperatorSum = 0xfe
	OperatorCmp = 0xff
)

type Script []uint8

func NewThresholdScript(threshold uint8) Script {
	return Script{OperatorCmp, OperatorSum, threshold}
}

func (s Script) VerifyFormat() error {
	if len(s) != 3 {
		return fmt.Errorf("invalid script length %d", len(s))
	}
	if s[0] != OperatorCmp || s[1] != OperatorSum {
		return fmt.Errorf("invalid script operators %d %d", s[0], s[1])
	}
	if s[2] > Operator64 {
		return fmt.Errorf("invalid script threshold %d", s[2])
	}
	return nil
}

func (s Script) Validate(sum int) error {
	err := s.VerifyFormat()
	if err != nil {
		return err
	}
	if sum < int(s[2]) {
		return fmt.Errorf("invalid signature keys %d %d", sum, s[2])
	}
	return nil
}

func (s Script) String() string {
	return hex.EncodeToString(s[:])
}

func (s Script) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(s.String())), nil
}

func (s *Script) UnmarshalJSON(b []byte) error {
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	data, err := hex.DecodeString(string(unquoted))
	if err != nil {
		return err
	}
	*s = data
	return nil
}
