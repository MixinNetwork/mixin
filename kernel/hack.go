package kernel

const (
	mainnetConsensusReferenceForkAt       = uint64(1736208000000000000)
	mainnetConsensusNodeRemovalTimeForkAt = uint64(1706400000000000000)
	mainnetMintDayGapSkipForkBatch        = uint64(1800)
	mainnetNodeRemovalHackSnapshotHash    = "b5a9ab66e3b5d24328f8f87bc38e90f0c426dc38413200bb8ecf7f5b8607a5f9"
	// mainnetGhostSeedReferenceForkAt activates mixing a recent transaction
	// hash into the deterministic ghost key seeds of protocol transactions
	// (mint, node remove), so their output keys can no longer be pre-computed
	// from public data long before the operation window.
	mainnetGhostSeedReferenceForkAt = uint64(1788220800000000000)
)
