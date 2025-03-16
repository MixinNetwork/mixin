package rpc

import (
	"encoding/json"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

type KernelInfo struct {
	Consensus crypto.Hash `json:"consensus"`
	Timestamp time.Time   `json:"timestamp"`
	Mint      struct {
		PoolSize common.Integer `json:"pool"`
	} `json:"mint"`
	Sequencer struct {
		Height    uint64      `json:"height"`
		Sequence  uint64      `json:"sequence"`
		Timestamp time.Time   `json:"timestamp"`
		Hash      crypto.Hash `json:"hash"`
	}
}

func GetInfo(rpc string) (*KernelInfo, error) {
	raw, err := CallMixinRPC(rpc, "getinfo", []any{})
	if err != nil || raw == nil {
		return nil, err
	}
	var info KernelInfo
	err = json.Unmarshal(raw, &info)
	if err != nil {
		panic(string(raw))
	}
	return &info, nil
}
