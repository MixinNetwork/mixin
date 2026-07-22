package kernel

import (
	"time"

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
	// calculation forever, but if this node continues to work after a long time,
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
	now := clock.NowUnixNano()
	now, ready := prepareNodeRemovalTime(now, node.Epoch)
	if !ready {
		return nil
	}
	rn := node.removingOrSlashingNodeAt(now)
	if rn == nil {
		return nil
	}
	if rn.IdForNetwork == id {
		return rn
	}
	return nil
}

func prepareNodeRemovalTime(now, epoch uint64) (uint64, bool) {
	if now < epoch {
		return 0, false
	}
	since := now - epoch
	h := int(since / uint64(time.Hour) % 24)
	b, e := config.KernelNodeAcceptTimeBegin, config.KernelNodeAcceptTimeEnd
	if h <= e && h+12 < b {
		return 0, false
	}
	if h > e && h < b+12 {
		return 0, false
	}
	// Keep this at the exact window boundary. Adding an offset can make an
	// early removal visible here and incorrectly advance the prediction to the
	// next accepted node while certificates for the first removal are in flight.
	since = since/OneDay*OneDay + uint64(b)*uint64(time.Hour)
	now = epoch + since
	return now, true
}

func (node *Node) usePredictiveNodeRemovalSignerSet(timestamp uint64) bool {
	return node.networkId.String() != config.KernelNetworkId ||
		timestamp >= mainnetConsensusNodeRemovalSignerSetForkAt
}

// removingOrSlashingNodeAt returns the node whose removal is already
// predictable at the beginning of the node-operation window containing
// timestamp. Using the window boundary, rather than the current graph head,
// keeps the result stable after the removal snapshot is finalized.
//
// FIXME: The prediction stops when the node-operation window closes. If a
// removal finalizes near that boundary but has not propagated to every node,
// snapshots timestamped after the window can again derive different signer
// vectors. A future consensus change should keep the signer set fixed across
// a deterministic epoch, for example until the next operation window.
// If node A is elected to do the removal, and it finalized the node removal
// snapshot, then the node goes offline and the snapshot never broadcasted
// and when the node A back online?
func (node *Node) removingOrSlashingNodeAt(timestamp uint64) *CNode {
	if timestamp < node.Epoch || !node.checkConsensusAcceptHour(timestamp) {
		return nil
	}
	since := timestamp - node.Epoch
	start := node.Epoch + since/OneDay*OneDay +
		uint64(config.KernelNodeAcceptTimeBegin)*uint64(time.Hour)
	rn, err := node.checkRemovePossibility(crypto.Hash{}, start, nil)
	if err != nil {
		return nil
	}
	return rn
}
