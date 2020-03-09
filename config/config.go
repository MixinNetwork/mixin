package config

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	Debug        = true
	BuildVersion = "v0.7.2-BUILD_VERSION"

	MainnetId                  = "6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997"
	SnapshotRoundGap           = uint64(3 * time.Second)
	SnapshotReferenceThreshold = 10
	SnapshotSyncRoundThreshold = 100
	TransactionMaximumSize     = 1024 * 1024
	WithdrawalClaimFee         = "0.0001"

	KernelMintTimeBegin = 7
	KernelMintTimeEnd   = 9

	KernelNodeAcceptTimeBegin     = 13
	KernelNodeAcceptTimeEnd       = 19
	KernelNodePledgePeriodMinimum = 12 * time.Hour
	KernelNodeAcceptPeriodMinimum = 12 * time.Hour
	KernelNodeAcceptPeriodMaximum = 7 * 24 * time.Hour
)

type custom struct {
	Environment    string        `json:"environment"`
	Signer         crypto.Key    `json:"signer"`
	Listener       string        `json:"listener"`
	MaxCacheSize   int           `json:"max-cache-size"`
	ElectionTicker int           `json:"election-ticker"`
	ConsensusOnly  bool          `json:"consensus-only"`
	CacheTTL       time.Duration `json:"cache-ttl"`
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
	var cm map[string]interface{}
	err = json.Unmarshal(f, &cm)
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
	if config.ElectionTicker == 0 {
		config.ElectionTicker = 700
	}
	if _, found := cm["consensus-only"]; !found {
		config.ConsensusOnly = true
	}
	Custom = &config
	return nil
}
