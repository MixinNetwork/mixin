package config

import (
	"io/ioutil"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/pelletier/go-toml"
)

const (
	Debug        = true
	BuildVersion = "v0.7.29-BUILD_VERSION"

	MainnetId                  = "6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997"
	SnapshotRoundGap           = uint64(3 * time.Second)
	SnapshotReferenceThreshold = 10
	SnapshotSyncRoundThreshold = 100
	SnapshotRoundSize          = 200
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
	Node struct {
		Signer               crypto.Key `toml:"-"`
		SignerStr            string     `toml:"signer-key"`
		ConsensusOnly        bool       `toml:"consensus-only"`
		KernelOprationPeriod int        `toml:"kernel-operation-period"`
		MemoryCacheSize      int        `toml:"memory-cache-size"`
		CacheTTL             int        `toml:"cache-ttl"`
		RingCacheSize        uint64     `toml:"ring-cache-size"`
		RingFinalSize        uint64     `toml:"ring-final-size"`
	} `toml:"node"`
	Network struct {
		Listener string `toml:"listener"`
	} `toml:"network"`
	RPC struct {
		Runtime bool `toml:"runtime"`
	} `toml:"rpc"`
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
	err = toml.Unmarshal(f, &config)
	if err != nil {
		return err
	}
	key, err := crypto.KeyFromString(config.Node.SignerStr)
	if err != nil {
		return err
	}
	config.Node.Signer = key
	if config.Node.KernelOprationPeriod == 0 {
		config.Node.KernelOprationPeriod = 700
	}
	if config.Node.MemoryCacheSize == 0 {
		config.Node.MemoryCacheSize = 1024 * 16
	}
	if config.Node.CacheTTL == 0 {
		config.Node.CacheTTL = 3600 * 2
	}
	if config.Node.RingCacheSize == 0 {
		config.Node.RingCacheSize = 1024 * 1024
	}
	if config.Node.RingFinalSize == 0 {
		config.Node.RingFinalSize = 1024 * 1024 * 16
	}
	Custom = &config
	return nil
}
