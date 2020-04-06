package config

import (
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
	Environment    string        `toml:"environment"`
	Signer         crypto.Key    `toml:"signer"`
	Listener       string        `toml:"listener"`
	MaxCacheSize   int           `toml:"max-cache-size"`
	RingCacheSize  uint64        `toml:"ring-cache-size"`
	RingFinalSize  uint64        `toml:"ring-final-size"`
	ElectionTicker int           `toml:"election-ticker"`
	ConsensusOnly  bool          `toml:"consensus-only"`
	CacheTTL       time.Duration `toml:"cache-ttl"`
	RPCRuntime     bool          `toml:"rpc-runtime"`
}

var Custom *custom

func Initialize(file string) error {
	if Custom != nil {
		return nil
	}
	f, err := toml.LoadFile(file)
	if err != nil {
		return err
	}
	document, err := toml.Marshal(f.ToMap())
	if err != nil {
		return err
	}
	var config custom
	err = toml.Unmarshal(document, &config)
	if err != nil {
		return err
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 3600 * 2
	}
	if config.MaxCacheSize == 0 {
		config.MaxCacheSize = 1024 * 16
	}
	if config.RingCacheSize == 0 {
		config.RingCacheSize = 1024 * 1024
	}
	if config.RingFinalSize == 0 {
		config.RingFinalSize = 1024 * 1024 * 16
	}
	if config.ElectionTicker == 0 {
		config.ElectionTicker = 700
	}
	if _, found := f.Get("consensus-only").(bool); !found {
		config.ConsensusOnly = true
	}
	Custom = &config
	return nil
}
