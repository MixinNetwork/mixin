package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) handleSnapshotInput(s *common.Snapshot) error {
	defer node.Graph.UpdateFinalCache(node.IdForNetwork)

	if node.verifyFinalization(s.Timestamp, s.Signatures) {
		err := node.tryToStartNewRound(s)
		if err != nil {
			return node.queueSnapshotOrPanic(s, true)
		}
		tx, err := node.checkFinalSnapshotTransaction(s)
		if err != nil {
			return node.queueSnapshotOrPanic(s, true)
		} else if tx == nil {
			return nil
		}
		return node.handleSyncFinalSnapshot(s, tx)
	}

	tx, err := node.checkCacheSnapshotTransaction(s)
	if err != nil {
		return node.queueSnapshotOrPanic(s, false)
	} else if tx == nil {
		return nil
	}

	if !node.CheckCatchUpWithPeers() && !node.checkInitialAcceptSnapshot(s, tx) {
		time.Sleep(100 * time.Millisecond)
		return node.queueSnapshotOrPanic(s, false)
	}

	if s.NodeId == node.IdForNetwork {
		if len(s.Signatures) == 0 {
			return node.signSelfSnapshot(s, tx)
		}
		return node.collectSelfSignatures(s, tx)
	}

	return node.verifyExternalSnapshot(s, tx)
}

func (node *Node) signSnapshot(s *common.Snapshot) {
	s.Hash = s.PayloadHash()
	sig := node.Signer.PrivateSpendKey.Sign(s.Hash[:])
	osigs := node.SnapshotsPool[s.Hash]
	for _, o := range osigs {
		if o.String() == sig.String() {
			panic("should never be here")
		}
	}
	node.SnapshotsPool[s.Hash] = append(osigs, &sig)
	node.SignaturesPool[s.Hash] = &sig

	key := append(s.Hash[:], sig[:]...)
	key = append(key, node.Signer.PublicSpendKey[:]...)
	hash := "KERNEL:SIGNATURE:" + crypto.NewHash(key).String()
	node.cacheStore.Set([]byte(hash), []byte{1})
}

func (node *Node) startNewRound(s *common.Snapshot, cache *CacheRound) (*FinalRound, error) {
	if s.RoundNumber != cache.Number+1 {
		panic("should never be here")
	}
	final := cache.asFinal()
	if final == nil {
		return nil, fmt.Errorf("self cache snapshots not collected yet")
	}
	if s.References.Self != final.Hash {
		return nil, fmt.Errorf("self cache snapshots not match yet")
	}

	external, err := node.persistStore.ReadRound(s.References.External)
	if err != nil {
		return nil, err
	}
	if external == nil {
		return nil, fmt.Errorf("external round %s not collected yet", s.References.External)
	}
	if final.NodeId == external.NodeId {
		return nil, nil
	}
	if !node.genesisNodesMap[external.NodeId] && external.Number < 7+config.SnapshotReferenceThreshold {
		return nil, nil
	}

	link, err := node.persistStore.ReadLink(s.NodeId, external.NodeId)
	if external.Number >= link {
		return final, err
	}
	return nil, err
}

func (node *Node) assignNewGraphRound(final *FinalRound, cache *CacheRound) {
	if final.NodeId != cache.NodeId {
		panic(fmt.Errorf("should never be here %s %s", final.NodeId, cache.NodeId))
	}
	node.Graph.CacheRound[final.NodeId] = cache
	node.Graph.FinalRound[final.NodeId] = final
	if history := node.Graph.RoundHistory[final.NodeId]; len(history) == 0 && final.Number == 0 {
		node.Graph.RoundHistory[final.NodeId] = append(node.Graph.RoundHistory[final.NodeId], final.Copy())
	} else if n := history[len(history)-1].Number; n > final.Number {
		panic(fmt.Errorf("should never be here %d %d", n, final.Number))
	} else if n+1 < final.Number {
		panic(fmt.Errorf("should never be here %d %d", n, final.Number))
	} else if n+1 == final.Number {
		node.Graph.RoundHistory[final.NodeId] = append(node.Graph.RoundHistory[final.NodeId], final.Copy())
	}
}

func (node *Node) CacheVerify(snap crypto.Hash, sig crypto.Signature, pub crypto.Key) bool {
	key := append(snap[:], sig[:]...)
	key = append(key, pub[:]...)
	hash := "KERNEL:SIGNATURE:" + crypto.NewHash(key).String()
	value := node.cacheStore.Get(nil, []byte(hash))
	if len(value) == 1 {
		return value[0] == byte(1)
	}
	valid := pub.Verify(snap[:], sig)
	if valid {
		node.cacheStore.Set([]byte(hash), []byte{1})
	} else {
		node.cacheStore.Set([]byte(hash), []byte{0})
	}
	return valid
}

func (node *Node) checkInitialAcceptSnapshotWeak(s *common.Snapshot) bool {
	pledge := node.ConsensusPledging
	if pledge == nil {
		return false
	}
	if node.genesisNodesMap[s.NodeId] {
		return false
	}
	if s.NodeId != pledge.Signer.Hash().ForNetwork(node.networkId) {
		return false
	}
	return s.RoundNumber == 0
}

func (node *Node) checkInitialAcceptSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) bool {
	if node.Graph.FinalRound[s.NodeId] != nil {
		return false
	}
	return node.checkInitialAcceptSnapshotWeak(s) && tx.TransactionType() == common.TransactionTypeNodeAccept
}

func (node *Node) queueSnapshotOrPanic(s *common.Snapshot, finalized bool) error {
	err := node.persistStore.QueueAppendSnapshot(node.IdForNetwork, s, finalized)
	if err != nil {
		panic(err)
	}
	return nil
}

func (node *Node) clearAndQueueSnapshotOrPanic(s *common.Snapshot) error {
	delete(node.SnapshotsPool, s.Hash)
	delete(node.SignaturesPool, s.Hash)
	node.removeFromCache(s)
	return node.queueSnapshotOrPanic(&common.Snapshot{
		NodeId:      s.NodeId,
		Transaction: s.Transaction,
	}, false)
}

func (node *Node) verifyFinalization(timestamp uint64, sigs []*crypto.Signature) bool {
	consensusThreshold := node.ConsensusBase(timestamp)*2/3 + 1
	return len(sigs) >= consensusThreshold
}
