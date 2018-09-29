package config

import "time"

const (
	SnapshotRoundGap       = uint64(3 * time.Second)
	TransactionMaximumSize = 1024 * 1024
)
