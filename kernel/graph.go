package kernel

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (chain *Chain) startNewRound(s *common.Snapshot, cache *CacheRound, allowDummy bool) (*FinalRound, bool, error) {
	if chain.ChainId != cache.NodeId {
		panic("should never be here")
	}
	if chain.ChainId != s.NodeId {
		panic("should never be here")
	}
	if s.RoundNumber != cache.Number+1 {
		panic("should never be here")
	}
	final := cache.asFinal()
	if final == nil {
		return nil, false, fmt.Errorf("self cache snapshots not collected yet %s %d", s.NodeId, s.RoundNumber)
	}
	if s.References.Self != final.Hash {
		return nil, false, fmt.Errorf("self cache snapshots not match yet %s %s", s.NodeId, s.References.Self)
	}

	finalized := chain.node.verifyFinalization(s)
	external, err := chain.persistStore.ReadRound(s.References.External)
	if err != nil {
		return nil, false, err
	}
	if external == nil && finalized && allowDummy {
		return final, true, nil
	}
	if external == nil {
		return nil, false, fmt.Errorf("external round %s not collected yet", s.References.External)
	}
	if final.NodeId == external.NodeId {
		return nil, false, nil
	}
	if !chain.node.genesisNodesMap[external.NodeId] && external.Number < 7+config.SnapshotReferenceThreshold {
		return nil, false, nil
	}
	if !finalized {
		externalChain := chain.node.GetOrCreateChain(external.NodeId)
		if external.Number+config.SnapshotSyncRoundThreshold < externalChain.State.FinalRound.Number {
			return nil, false, fmt.Errorf("external reference %s too early %d %d", s.References.External, external.Number, externalChain.State.FinalRound.Number)
		}
		if external.Timestamp > s.Timestamp {
			return nil, false, fmt.Errorf("external reference later than snapshot time %f", time.Duration(external.Timestamp-s.Timestamp).Seconds())
		}
		threshold := external.Timestamp + config.SnapshotReferenceThreshold*config.SnapshotRoundGap*64
		best := chain.node.determinBestRound(s.NodeId, s.Timestamp)
		if best != nil && threshold < best.Start {
			return nil, false, fmt.Errorf("external reference %s too early %s:%d %f", s.References.External, best.NodeId, best.Number, time.Duration(best.Start-threshold).Seconds())
		}
	}
	link, err := chain.persistStore.ReadLink(s.NodeId, external.NodeId)
	if external.Number < link {
		return nil, false, err
	}
	if external.NodeId == chain.ChainId {
		if l := chain.State.ReverseRoundLinks[s.NodeId]; external.Number < l {
			return nil, false, fmt.Errorf("external reverse reference %s %d %d", s.NodeId, external.Number, l)
		}
		chain.State.ReverseRoundLinks[s.NodeId] = external.Number
	}
	return final, false, err
}

func (chain *Chain) assignNewGraphRound(final *FinalRound, cache *CacheRound) {
	if chain.ChainId != cache.NodeId {
		panic("should never be here")
	}
	if chain.ChainId != final.NodeId {
		panic("should never be here")
	}

	chain.State.Lock()
	defer chain.State.Unlock()

	if final.NodeId != cache.NodeId {
		panic(fmt.Errorf("should never be here %s %s", final.NodeId, cache.NodeId))
	}
	chain.State.CacheRound = cache
	chain.State.FinalRound = final
	if history := chain.State.RoundHistory; len(history) == 0 && final.Number == 0 {
		chain.State.RoundHistory = append(chain.State.RoundHistory, final.Copy())
	} else if n := history[len(history)-1].Number; n > final.Number {
		panic(fmt.Errorf("should never be here %s %d %d", final.NodeId, final.Number, n))
	} else if n+1 < final.Number {
		panic(fmt.Errorf("should never be here %s %d %d", final.NodeId, final.Number, n))
	} else if n+1 == final.Number {
		chain.State.RoundHistory = append(chain.State.RoundHistory, final.Copy())
		chain.StepForward()
	}

	if final.End > chain.node.GraphTimestamp {
		chain.node.GraphTimestamp = final.End
	}

	rounds := chain.State.RoundHistory
	best := rounds[len(rounds)-1].Start
	threshold := config.SnapshotReferenceThreshold * config.SnapshotRoundGap * 64
	if rounds[0].Start+threshold > best && len(rounds) <= config.SnapshotReferenceThreshold {
		return
	}
	newRounds := make([]*FinalRound, 0)
	for _, r := range rounds {
		if r.Start+threshold <= best {
			continue
		}
		newRounds = append(newRounds, r)
	}
	if rc := len(newRounds) - config.SnapshotReferenceThreshold; rc > 0 {
		newRounds = newRounds[rc:]
	}
	chain.State.RoundHistory = newRounds
}

func (node *Node) CacheVerify(snap crypto.Hash, sig crypto.Signature, pub crypto.Key) bool {
	key := append(snap[:], sig[:]...)
	key = append(key, pub[:]...)
	value := node.cacheStore.Get(nil, key)
	if len(value) == 1 {
		return value[0] == byte(1)
	}
	valid := pub.Verify(snap[:], sig)
	if valid {
		node.cacheStore.Set(key, []byte{1})
	} else {
		node.cacheStore.Set(key, []byte{0})
	}
	return valid
}

func (node *Node) CacheVerifyCosi(snap crypto.Hash, sig *crypto.CosiSignature, publics []*crypto.Key, threshold int) bool {
	if snap.String() == "b3ea56de6124ad2f3ad1d48f2aff8338b761e62bcde6f2f0acba63a32dd8eecc" &&
		sig.String() == "dbb0347be24ecb8de3d66631d347fde724ff92e22e1f45deeb8b5d843fd62da39ca8e39de9f35f1e0f7336d4686917983470c098edc91f456d577fb18069620f000000003fdfe712" {
		// FIXME this is a hack to fix the large round gap around node remove snapshot
		// and a bug in too recent external reference, e.g. bare final round
		return true
	}
	key := sig.Signature[:]
	key = append(snap[:], key...)
	for _, pub := range publics {
		key = append(key, pub[:]...)
	}
	tbuf := make([]byte, 8)
	binary.BigEndian.PutUint64(tbuf, uint64(threshold))
	key = append(key, tbuf...)
	binary.BigEndian.PutUint64(tbuf, sig.Mask)
	key = append(key, tbuf...)
	value := node.cacheStore.Get(nil, key)
	if len(value) == 1 {
		return value[0] == byte(1)
	}
	err := sig.FullVerify(publics, threshold, snap[:])
	if err != nil {
		logger.Verbosef("CacheVerifyCosi(%s, %d, %d) ERROR %s\n", snap, len(publics), threshold, err.Error())
		node.cacheStore.Set(key, []byte{0})
	} else {
		node.cacheStore.Set(key, []byte{1})
	}
	return err == nil
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
	chain := node.GetOrCreateChain(s.NodeId)
	if chain.State.FinalRound != nil {
		return false
	}
	return node.checkInitialAcceptSnapshotWeak(s) && tx.TransactionType() == common.TransactionTypeNodeAccept
}

func (chain *Chain) queueSnapshotOrPanic(peerId crypto.Hash, s *common.Snapshot) error {
	if chain.ChainId != s.NodeId {
		panic("should never be here")
	}
	err := chain.AppendCacheSnapshot(peerId, s)
	if err != nil {
		panic(err)
	}
	return nil
}

func (chain *Chain) clearAndQueueSnapshotOrPanic(s *common.Snapshot) error {
	if chain.ChainId != s.NodeId {
		panic("should never be here")
	}
	delete(chain.CosiVerifiers, s.Hash)
	delete(chain.CosiAggregators, s.Hash)
	delete(chain.CosiAggregators, s.Transaction)
	return chain.queueSnapshotOrPanic(chain.ChainId, &common.Snapshot{
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
	if node.checkInitialAcceptSnapshotWeak(s) {
		publics = append(publics, &node.ConsensusPledging.Signer.PublicSpendKey)
	}
	base := node.ConsensusThreshold(s.Timestamp)
	if node.CacheVerifyCosi(s.Hash, s.Signature, publics, base) {
		return true
	}
	if rr := node.ConsensusRemovedRecently(s.Timestamp); rr != nil {
		for i := range publics {
			pwr := append([]*crypto.Key{}, publics[:i]...)
			pwr = append(pwr, &rr.Signer.PublicSpendKey)
			pwr = append(pwr, publics[i:]...)
			if node.CacheVerifyCosi(s.Hash, s.Signature, pwr, base) {
				return true
			}
		}
	}
	return false
}

func (node *Node) legacyVerifyFinalization(timestamp uint64, sigs []*crypto.Signature) bool {
	return len(sigs) >= node.ConsensusThreshold(timestamp)
}
