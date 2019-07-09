package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) CosiCommit(peerId crypto.Hash, s *common.Snapshot) error {
	return nil
}

func (node *Node) CosiAggregateCommitments(peerId crypto.Hash, snap crypto.Hash, commitment crypto.Key, wantTx bool) error {
	return nil
}

func (node *Node) CosiChallenge(peerId crypto.Hash, snap crypto.Hash, cosi crypto.CosiSignature, ver *common.VersionedTransaction) error {
	return nil
}

func (node *Node) CosiAggregateResponses(peerId crypto.Hash, snap crypto.Hash, response [32]byte) error {
	return nil
}
