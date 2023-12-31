package kernel

import (
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
)

const (
	// All nodes should ensure consistent snapshots to keep the round space small.
	SlashReasonRoundSpace = 0x1

	// After a kernel node is pledged successfully, it must be accepted timely.
	SlashReasonLateAccept = 0x2

	// The payee should spend the node removal output as soon as possible.
	// The slash could be done by kernel nodes directly when the consumption
	// of the output happens.
	SlashReasonLateRemove = 0x3

	// A sequencer node must produce consistent topology with witness signature.
	// Light node could provide evidence of inconsistent topology.
	SlashReasonInconsistentWitness = 0x4

	// A node produces snapshot x in round A, then announces round B, which
	// references round A with only snapshot x in it. However during round A,
	// the node also announces snapshot y and gathered enough signatures, but
	// this snapshot y is never delivered to other nodes after the finalization
	// then the round A is considered stale if anyone could provide evidence of
	// the existence of the finalized snapshot y. Light node could do this job.
	//
	// Another kind of stale snapshot could directly happen in round A without
	// the announcement of round B. The node just announces x and keeps y, then
	// stops all future works. This can already be punished by large space and
	// mint rewards, but we still need to add more measurements for this behavior.
	// Because this stale snapshot y must be excluded from mint works
	// calculation forever, but if this node contiues to work after a long time,
	// and the failure to identity this stale snapshot timely, could result in
	// the full node sync failure of a fresh boot node. So the best punishment
	// is to remove this node, together with the large round space check.
	SlashReasonStaleSnapshot = 0x5
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
