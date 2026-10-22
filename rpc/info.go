package rpc

import (
	"encoding/json"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

type KernelInfo struct {
	Consensus crypto.Hash `json:"consensus"`
	Timestamp string      `json:"timestamp"`
	Mint      struct {
		PoolSize common.Integer `json:"pool"`
	} `json:"mint"`
}

func GetInfo(rpc string) (*KernelInfo, error) {
	raw, err := CallMixinRPC(rpc, "getinfo", []any{})
	if err != nil || raw == nil {
		return nil, err
	}
	var info KernelInfo
	err = json.Unmarshal(raw, &info)
	if err != nil {
		return nil, fmt.Errorf("decode info response: %w", err)
	}
	return &info, nil
}
