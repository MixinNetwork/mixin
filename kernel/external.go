package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) verifyExternalSnapshot(s *common.Snapshot) error {
	if s.NodeId == node.IdForNetwork || len(s.Signatures) != 1 {
		panic("should never be here")
	}
	if len(node.SnapshotsPool[s.Hash]) > 0 || node.SignaturesPool[s.Hash] != nil {
		return nil
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if s.RoundNumber < cache.Number {
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return node.queueSnapshotOrPanic(s, false)
	}
	if s.RoundNumber == cache.Number {
		if !s.References.Equal(cache.References) {
			return nil
		}
	}
	if s.RoundNumber == cache.Number+1 {
		if round, err := node.startNewRound(s, cache); err != nil {
			return node.queueSnapshotOrPanic(s, false)
		} else if round == nil {
			return nil
		} else {
			final = round
		}
		cache = &CacheRound{
			NodeId:     s.NodeId,
			Number:     s.RoundNumber,
			Timestamp:  s.Timestamp,
			References: s.References,
		}
		err := node.store.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
	}

	node.Graph.CacheRound[s.NodeId] = cache
	node.Graph.FinalRound[s.NodeId] = final
	node.signSnapshot(s)
	s.Signatures = []*crypto.Signature{node.SignaturesPool[s.Hash]}
	return node.Peer.SendSnapshotMessage(s.NodeId, s, 0)
}
