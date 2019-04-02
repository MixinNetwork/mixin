package config

import "time"

const (
	Debug        = true
	BuildVersion = "v0.1.14-BUILD_VERSION"

	SnapshotRoundGap           = uint64(3 * time.Second)
	SnapshotReferenceThreshold = 10
	TransactionMaximumSize     = 1024 * 1024
	CacheTTL                   = 2 * time.Hour
)
