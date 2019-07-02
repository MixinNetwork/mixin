package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) verifyExternalSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	if s.NodeId == node.IdForNetwork || len(s.Signatures) != 1 {
		panic(fmt.Errorf("should never be here %s %s %d", node.IdForNetwork, s.NodeId, len(s.Signatures)))
	}
	if len(node.SnapshotsPool[s.Hash]) > 0 || node.SignaturesPool[s.Hash] != nil {
		return nil
	}
	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if s.Timestamp > uint64(time.Now().UnixNano())+threshold {
		return nil
	}
	if s.Timestamp+threshold*2 < node.Graph.GraphTimestamp {
		return nil
	}

	if node.checkInitialAcceptSnapshot(s, tx) {
		node.signSnapshot(s)
		s.Signatures = []*crypto.Signature{node.SignaturesPool[s.Hash]}
		return node.Peer.SendSnapshotMessage(s.NodeId, s, 0)
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if s.RoundNumber < cache.Number {
		return nil
	}
	if s.RoundNumber > cache.Number+1 {
		return node.queueSnapshotOrPanic(s, false)
	}
	if s.Timestamp <= final.Start+config.SnapshotRoundGap {
		return nil
	}
	if s.RoundNumber == cache.Number {
		if !s.References.Equal(cache.References) {
			return nil
		}
	}
	if s.RoundNumber == cache.Number+1 {
		if round, err := node.startNewRound(s, cache); err != nil {
			logger.Verbosef("ERROR verifyExternalSnapshot %s %d %s %s %s\n", s.NodeId, s.RoundNumber, s.References.Self, s.References.External, err.Error())
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
		err := node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
		if err != nil {
			panic(err)
		}
		node.CachePool[s.NodeId] = make([]*common.Snapshot, 0)
	}
	node.assignNewGraphRound(final, cache)

	if !cache.ValidateSnapshot(s, false) {
		return nil
	}
	if node.checkCacheExist(s) {
		return nil
	}

	node.signSnapshot(s)
	s.Signatures = []*crypto.Signature{node.SignaturesPool[s.Hash]}
	node.CachePool[s.NodeId] = append(node.CachePool[s.NodeId], s)
	return node.Peer.SendSnapshotMessage(s.NodeId, s, 0)
}
