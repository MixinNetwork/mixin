package config

import "time"

const (
	Debug                  = true
	SnapshotRoundGap       = uint64(3 * time.Second)
	TransactionMaximumSize = 1024 * 1024
	CacheTTL               = 2 * time.Hour
)
