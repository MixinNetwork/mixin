package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) handleSnapshotInput(s *common.Snapshot) error {
	defer node.Graph.UpdateFinalCache()

	if node.verifyFinalization(s.Signatures) {
		valid, err := node.checkFinalSnapshotTransaction(s)
		if err != nil {
			return node.queueSnapshotOrPanic(s, true)
		} else if !valid {
			return nil
		}
		return node.handleSyncFinalSnapshot(s)
	}

	tx, err := node.checkCacheSnapshotTransaction(s)
	if err != nil {
		return node.queueSnapshotOrPanic(s, false)
	} else if tx == nil {
		return nil
	}
	if s.NodeId == node.IdForNetwork {
		if len(s.Signatures) == 0 {
			return node.signSelfSnapshot(s, tx)
		}
		return node.collectSelfSignatures(s)
	}

	return node.verifyExternalSnapshot(s)
}

func (node *Node) signSnapshot(s *common.Snapshot) {
	s.Hash = s.PayloadHash()
	sig := node.Account.PrivateSpendKey.Sign(s.Hash[:])
	osigs := node.SnapshotsPool[s.Hash]
	for _, o := range osigs {
		if o.String() == sig.String() {
			panic("should never be here")
		}
	}
	node.SnapshotsPool[s.Hash] = append(osigs, &sig)
	node.SignaturesPool[s.Hash] = &sig
}

func (node *Node) queueSnapshotOrPanic(s *common.Snapshot, finalized bool) error {
	err := node.store.QueueAppendSnapshot(node.IdForNetwork, s, finalized)
	if err != nil {
		panic(err)
	}
	return nil
}

func (node *Node) clearAndQueueSnapshotOrPanic(s *common.Snapshot) error {
	delete(node.SnapshotsPool, s.PayloadHash())
	delete(node.SignaturesPool, s.PayloadHash())
	return node.queueSnapshotOrPanic(&common.Snapshot{
		NodeId:      s.NodeId,
		Transaction: s.Transaction,
	}, false)
}

func (node *Node) verifyFinalization(sigs []*crypto.Signature) bool {
	consensusThreshold := len(node.ConsensusNodes) * 2 / 3
	return len(sigs) > consensusThreshold
}
