package config

import "time"

const (
	Debug        = true
	BuildVersion = "v0.2.10-BUILD_VERSION"

	MainnetId                  = "6430225c42bb015b4da03102fa962e4f4ef3969e03e04345db229f8377ef7997"
	SnapshotRoundGap           = uint64(3 * time.Second)
	SnapshotReferenceThreshold = 10
	TransactionMaximumSize     = 1024 * 1024
	CacheTTL                   = 2 * time.Hour
	WithdrawalClaimFee         = "0.0001"
)
