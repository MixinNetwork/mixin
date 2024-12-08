package config

import (
	"os"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/pelletier/go-toml"
)

const (
	Debug           = true
	BuildVersion    = "v0.18.20-BUILD_VERSION"
	KernelNetworkId = "74c6cdb7d51af57037faa1f5544f8331ced001df5964331911ca51385993b375"

	SnapshotRoundGap           = uint64(3 * time.Second)
	SnapshotReferenceThreshold = 10
	SnapshotSyncRoundThreshold = 100
	SnapshotRoundSize          = 200

	CheckpointDuration        = 10 * time.Minute
	CheckpointPunishmentGrade = 7

	TransactionMaximumSize = 1024 * 1024 * 4
	WithdrawalClaimFee     = "0.0001"
	GossipSize             = 3

	KernelMinimumNodesCount = 7

	KernelMintTimeBegin = 7
	KernelMintTimeEnd   = 9

	KernelNodeAcceptTimeBegin     = 13
	KernelNodeAcceptTimeEnd       = 19
	KernelNodePledgePeriodMinimum = 12 * time.Hour
	KernelNodeAcceptPeriodMinimum = 12 * time.Hour
	KernelNodeAcceptPeriodMaximum = 7 * 24 * time.Hour
)

type Custom struct {
	Node struct {
		Signer               crypto.Key `toml:"-"`
		SignerStr            string     `toml:"signer-key"`
		KernelOprationPeriod int        `toml:"kernel-operation-period"`
		MemoryCacheSize      int        `toml:"memory-cache-size"`
		CacheTTL             int        `toml:"cache-ttl"`
	} `toml:"node"`
	Storage struct {
		ValueLogGC          bool `toml:"value-log-gc"`
		MaxCompactionLevels int  `toml:"max-compaction-levels"`
	} `toml:"storage"`
	P2P struct {
		Port    int      `toml:"port"`
		Seeds   []string `toml:"seeds"`
		Relayer bool     `toml:"relayer"`
		Metric  bool     `toml:"metric"`
	} `toml:"p2p"`
	RPC struct {
		Port         int  `toml:"port"`
		Runtime      bool `toml:"runtime"`
		ObjectServer bool `toml:"object-server"`
	} `toml:"rpc"`
	Dev struct {
		Port int `toml:"port"`
	} `toml:"dev"`
}

func Initialize(file string) (*Custom, error) {
	f, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var config Custom
	err = toml.Unmarshal(f, &config)
	if err != nil {
		return nil, err
	}
	key, err := crypto.KeyFromString(config.Node.SignerStr)
	if err != nil {
		return nil, err
	}
	config.Node.Signer = key
	if config.Node.KernelOprationPeriod == 0 {
		config.Node.KernelOprationPeriod = 700
	}
	if config.Node.MemoryCacheSize == 0 {
		config.Node.MemoryCacheSize = 1024 * 4
	}
	if config.Node.CacheTTL == 0 {
		config.Node.CacheTTL = 3600 * 2
	}
	return &config, nil
}
