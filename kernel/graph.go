package kernel

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

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
	if !node.verifyFinalization(s) {
		if external.Number+config.SnapshotSyncRoundThreshold < node.Graph.FinalRound[external.NodeId].Number {
			return nil, fmt.Errorf("external reference %s too early %d %d", s.References.External, external.Number, node.Graph.FinalRound[external.NodeId].Number)
		}
		if external.Timestamp > s.Timestamp+config.SnapshotRoundGap {
			return nil, fmt.Errorf("external reference later than snapshot time %f", time.Duration(external.Timestamp-s.Timestamp).Seconds())
		}
		threshold := external.Timestamp + config.SnapshotReferenceThreshold*config.SnapshotRoundGap*64
		for _, rounds := range node.Graph.RoundHistory {
			r := rounds[0]
			if r.NodeId == s.NodeId {
				continue
			}
			if len(rounds) > config.SnapshotReferenceThreshold {
				r = rounds[len(rounds)-config.SnapshotReferenceThreshold]
			}
			if threshold < r.Start {
				return nil, fmt.Errorf("external reference %s too early %f", s.References.External, time.Duration(r.Start-external.Timestamp).Seconds())
			}
		}
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

func (node *Node) CacheVerifyCosi(snap crypto.Hash, sig *crypto.CosiSignature, publics []*crypto.Key, threshold int) bool {
	key := common.MsgpackMarshalPanic(sig)
	key = append(snap[:], key...)
	for _, pub := range publics {
		key = append(key, pub[:]...)
	}
	tbuf := make([]byte, 8)
	binary.BigEndian.PutUint64(tbuf, uint64(threshold))
	key = append(key, tbuf...)
	hash := "KERNEL:COSISIGNATURE:" + crypto.NewHash(key).String()
	value := node.cacheStore.Get(nil, []byte(hash))
	if len(value) == 1 {
		return value[0] == byte(1)
	}
	valid := sig.FullVerify(publics, threshold, snap[:])
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
	if s.NodeId != pledge.IdForNetwork(node.networkId) {
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

func (node *Node) queueSnapshotOrPanic(peerId crypto.Hash, s *common.Snapshot) error {
	time.Sleep(10 * time.Millisecond)
	err := node.persistStore.QueueAppendSnapshot(peerId, s, false)
	if err != nil {
		panic(err)
	}
	return nil
}

func (node *Node) clearAndQueueSnapshotOrPanic(s *common.Snapshot) error {
	delete(node.CosiVerifiers, s.Hash)
	node.CosiAggregators.Delete(s.Hash)
	node.CosiAggregators.Delete(s.Transaction)
	return node.queueSnapshotOrPanic(node.IdForNetwork, &common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      s.NodeId,
		Transaction: s.Transaction,
	})
}

func (node *Node) verifyFinalization(s *common.Snapshot) bool {
	if s.Version == 0 {
		return node.legacyVerifyFinalization(s.Timestamp, s.Signatures)
	}
	if s.Version != common.SnapshotVersion || s.Signature == nil {
		return false
	}
	publics := node.ConsensusKeys(s.Timestamp)
	base := node.ConsensusThreshold(s.Timestamp)
	return node.CacheVerifyCosi(s.PayloadHash(), s.Signature, publics, base)
}

func (node *Node) legacyVerifyFinalization(timestamp uint64, sigs []*crypto.Signature) bool {
	return len(sigs) >= node.ConsensusThreshold(timestamp)
}
