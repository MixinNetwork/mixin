package kernel

import (
	"fmt"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

// Protocol transactions (mint, node remove) are rebuilt deterministically by
// every validator, so their output ghost keys historically derived from
// public data only: anyone could pre-compute the keys far in advance and
// permanently reserve them with dust outputs, because ghost key locks are
// written at queue validation time and are never released. Mixing a recent
// transaction hash into the seed keeps the rebuild deterministic for
// validators (the anchor is recorded in the transaction references) while
// remaining unpredictable until shortly before the operation window.

// ghostSeedReferenceEnabled reports whether protocol transaction ghost key
// seeds must mix a recent transaction reference at the given snapshot
// timestamp.
func (node *Node) ghostSeedReferenceEnabled(timestamp uint64) bool {
	if node.networkId.String() == config.KernelNetworkId && timestamp < mainnetGhostSeedReferenceForkAt {
		return false
	}
	return true
}

// protocolGhostSeed derives the deterministic output seed for a protocol
// transaction. When anchor is empty the derivation stays bit-compatible
// with the legacy scheme.
func protocolGhostSeed(input string, anchor crypto.Hash) []byte {
	if anchor.HasValue() {
		input += anchor.String()
	}
	si := crypto.Blake3Hash([]byte(input))
	return append(si[:], si[:]...)
}

// ghostSeedReference returns a recent finalized transaction hash to anchor a
// protocol transaction ghost seed on. It returns a zero hash when the scheme
// is not enabled yet at the given timestamp.
func (node *Node) ghostSeedReference(timestamp uint64) (crypto.Hash, error) {
	var zero crypto.Hash
	if !node.ghostSeedReferenceEnabled(timestamp) {
		return zero, nil
	}
	round := node.chain.State.FinalRound.Number
	snaps, err := node.persistStore.ReadSnapshotsForNodeRound(node.IdForNetwork, round)
	if err != nil {
		return zero, err
	}
	return snaps[0].Transactions[0], nil
}

// ghostSeedReferenceFromTx extracts the anchor transaction hash recorded in
// the references of a protocol transaction, right after the consensus
// reference. Validators use it to rebuild the exact same deterministic
// outputs. It returns a zero hash when the scheme is not enabled at the
// given timestamp.
func (node *Node) ghostSeedReferenceFromTx(refs []crypto.Hash, timestamp uint64) (crypto.Hash, error) {
	var zero crypto.Hash
	if !node.ghostSeedReferenceEnabled(timestamp) {
		return zero, nil
	}
	if len(refs) != 2 {
		return zero, fmt.Errorf("invalid ghost seed references count %d at %d", len(refs), timestamp)
	}
	anchor := refs[1]
	if !anchor.HasValue() {
		return zero, fmt.Errorf("invalid empty ghost seed reference at %d", timestamp)
	}
	return anchor, nil
}
