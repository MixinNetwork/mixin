package kernel

import (
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
)

const (
	SlashReasonRoundSpace          = 0x1
	SlashReasonLateAccept          = 0x2
	SlashReasonLateRemove          = 0x3
	SlashReasonInconsistentWitness = 0x4
)

// this returns the first to be removed node as normal, then if some node
// is being slashed, this will return it instead of the normal one, thus
// causes the removing or slashing node changes.
//
// a removing or slashing node is not able to produce works during the state,
// but whenever another slashing node with more priority proceeds it, it resumes
// working again, a large round space may happen.
//
// to make it equal and fair, the large round space slashing will cause zero
// loss, and the payee will get back the whole pledge. so the punishment to
// a removing or slashing node is only drastically mint decline.
func (node *Node) GetRemovingOrSlashingNode(id crypto.Hash) *CNode {
	now := uint64(clock.Now().Add(config.KernelNodeAcceptPeriodMinimum).UnixNano())
	rn, err := node.checkRemovePossibility(crypto.Hash{}, now, nil)
	if err != nil || rn == nil {
		return nil
	}
	if rn.IdForNetwork == id {
		return rn
	}
	return nil
}
