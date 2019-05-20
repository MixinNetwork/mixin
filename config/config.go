package config

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	Debug        = true
	BuildVersion = "v0.3.19-BUILD_VERSION"

	MainnetId                  = "6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997"
	SnapshotRoundGap           = uint64(3 * time.Second)
	SnapshotReferenceThreshold = 10
	TransactionMaximumSize     = 1024 * 1024
	WithdrawalClaimFee         = "0.0001"

	KernelMintTimeBegin = 7
	KernelMintTimeEnd   = 9

	KernelNodeAcceptTimeBegin        = 13
	KernelNodeAcceptTimeEnd          = 19
	KernelNodeOperationLockThreshold = 1 * time.Hour
	KernelNodePledgePeriodMinimum    = 12 * time.Hour
	KernelNodeAcceptPeriodMinimum    = 12 * time.Hour
	KernelNodeAcceptPeriodMaximum    = 7 * 24 * time.Hour
)

type custom struct {
	Signer       crypto.Key    `json:"signer"`
	Listener     string        `json:"listener"`
	MaxCacheSize int           `json:"max-cache-size"`
	CacheTTL     time.Duration `json:"cache-ttl"`
}

var Custom *custom

func Initialize(file string) error {
	if Custom != nil {
		return nil
	}
	f, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	var config custom
	err = json.Unmarshal(f, &config)
	if err != nil {
		return err
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 3600 * 2
	}
	if config.MaxCacheSize == 0 {
		config.MaxCacheSize = 1024 * 16
	}
	Custom = &config
	return nil
}
