package kernel

import (
	"encoding/hex"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

const (
	SystemVoteCheckpoint = 0
)

// we may not need this actually, because the snapshot
// should reference a block hash between 128 blocks to 512 blocks shorter
// we just change the snapshot round reference from external node route
// to the global blockchain block
//
// We still need this, it's very important to put this order
// in the snapshots topology checkpoint, because the sequencer
// is a independent loop from the cosi
type CheckpointVote struct {
	NodeId crypto.Hash
	Blocks [512]crypto.Hash
}

func isCheckpointVote(tx *common.VersionedTransaction) bool {
	if tx.TransactionType() != common.TransactionTypeSystemVote {
		panic(hex.EncodeToString(tx.Marshal()))
	}
	return tx.Extra[0] == SystemVoteCheckpoint && len(tx.Extra) == 32*513
}
