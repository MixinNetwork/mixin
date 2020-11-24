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

const (
	GraphOperationClassAtomic       = 0x00
	GraphOperationClassNormalLedger = 0x01

	MainnetNodeRemovalConsensusForkTimestamp = 1590000000000000000
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

	finalized := chain.verifyFinalization(s)
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
		best, err := chain.determinBestRound(s.Timestamp, external.NodeId)
		if err != nil {
			return nil, false, fmt.Errorf("external reference %s invalid %s", s.References.External, err)
		} else if best != nil && threshold < best.Start {
			return nil, false, fmt.Errorf("external reference %s too early %s:%d %f", s.References.External, best.NodeId, best.Number, time.Duration(best.Start-threshold).Seconds())
		}
	}
	if external.Number < chain.State.RoundLinks[external.NodeId] {
		return nil, false, err
	}
	link, err := chain.persistStore.ReadLink(s.NodeId, external.NodeId)
	if err != nil {
		return nil, false, err
	}
	if link != chain.State.RoundLinks[external.NodeId] {
		panic(fmt.Errorf("should never be here %s=>%s %d %d", chain.ChainId, external.NodeId, link, chain.State.RoundLinks[external.NodeId]))
	}
	chain.State.RoundLinks[external.NodeId] = external.Number

	return final, false, err
}

func (chain *Chain) updateEmptyHeadRound(m *CosiAction, cache *CacheRound, s *common.Snapshot) (bool, error) {
	if len(cache.Snapshots) != 0 {
		logger.Verbosef("ERROR cosiHandleFinalization malformated head round references not empty %s %v %d\n", m.PeerId, s, len(cache.Snapshots))
		return false, nil
	}
	if s.References.Self != cache.References.Self {
		logger.Verbosef("ERROR cosiHandleFinalization malformated head round references self diff %s %v %v\n", m.PeerId, s, cache.References)
		return false, nil
	}
	external, err := chain.persistStore.ReadRound(s.References.External)
	if err != nil || external == nil {
		logger.Verbosef("ERROR cosiHandleFinalization head round references external not ready yet %s %v %v\n", m.PeerId, s, cache.References)
		return false, err
	}
	link, err := chain.persistStore.ReadLink(cache.NodeId, external.NodeId)
	if err != nil || external.Number < link {
		return false, err
	}
	chain.State.RoundLinks[external.NodeId] = external.Number
	return true, nil
}

func (chain *Chain) assignNewGraphRound(final *FinalRound, cache *CacheRound) {
	if chain.ChainId != cache.NodeId {
		panic("should never be here")
	}
	if chain.ChainId != final.NodeId {
		panic("should never be here")
	}
	if final.Number+1 != cache.Number {
		panic("should never be here")
	}
	if final.NodeId != cache.NodeId {
		panic(fmt.Errorf("should never be here %s %s", final.NodeId, cache.NodeId))
	}

	chain.State.CacheRound = cache
	chain.State.FinalRound = final
	if final.End > chain.node.GraphTimestamp {
		chain.node.GraphTimestamp = final.End
	}

	rounds := chain.State.RoundHistory
	if len(rounds) == 0 && final.Number == 0 {
		logger.Printf("assign the first round %s %s\n", chain.node.IdForNetwork, chain.ChainId)
	} else if n := rounds[len(rounds)-1].Number; n == final.Number {
		return
	} else if n+1 != final.Number {
		panic(fmt.Errorf("should never be here %s %d %d", final.NodeId, final.Number, n))
	}

	rounds = append(rounds, final.Copy())
	chain.StepForward()

	threshold := config.SnapshotReferenceThreshold * config.SnapshotRoundGap * 64
	if rounds[0].Start+threshold > final.Start && len(rounds) <= config.SnapshotReferenceThreshold {
		chain.State.RoundHistory = rounds
		return
	}
	newRounds := make([]*FinalRound, 0)
	for _, r := range rounds {
		if r.Start+threshold <= final.Start {
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

// Nodes list change problem:
// 1. Node A gets snapshot S signed by enough nodes, including B, at time 10, and finalized but not broadcasted to others yet.
// Then node B is removed at time 9. Now A broadcasts S, and others will not be able to finalize S.
// Solution: Because A has the ACK of node B, then A should include B when challenge all others, then others will record the
// ACK timestamp of node B is time 10. So that if B has a conflict removal time of 9, then won't get ACKed at all.
// What if A doesn't include B in the challenge, then A may be found evial and slashed.
// Proof: With the solution, it's impossible to have B removed at 9, and S get finalized get 10. Because 2f+1 nodes know B ACK S
// at 10, then they won't accept removal of B at 9.
// 2. Node A initial accept snapshot I signed by enough nodes, at time 9, and finalized but not broadcasted to others yet.
// Then snapshot S is finalized at time 10. Now A broadcasts I, and others will not be able to finalize S.
// Solution: Now node A is evil and will be slashed.
// 3. Node A pledge snapshot finalized but not broadcasted on time.
// Solution: Evil and slash.

func (node *Node) CacheVerifyCosi(snap crypto.Hash, sig *crypto.CosiSignature, publics []*crypto.Key, threshold int) bool {
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

func (chain *Chain) queueActionOrPanic(m *CosiAction) error {
	if chain.ChainId != m.PeerId {
		panic("should never be here")
	}
	err := chain.AppendCosiAction(m)
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
	return chain.AppendSelfEmpty(&common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      s.NodeId,
		Transaction: s.Transaction,
	})
}

func (chain *Chain) verifyFinalization(s *common.Snapshot) bool {
	if s.Version == 0 {
		return chain.legacyVerifyFinalization(s.Timestamp, s.Signatures)
	}
	if s.Version != common.SnapshotVersion || s.Signature == nil {
		return false
	}
	publics := chain.ConsensusKeys(s.RoundNumber, s.Timestamp)
	base := chain.node.ConsensusThreshold(s.Timestamp)
	finalized := chain.node.CacheVerifyCosi(s.Hash, s.Signature, publics, base)
	if finalized || s.Timestamp > MainnetNodeRemovalConsensusForkTimestamp {
		return finalized
	}

	timestamp := s.Timestamp - uint64(config.KernelNodeAcceptPeriodMinimum)
	publics = chain.ConsensusKeys(s.RoundNumber, timestamp)
	base = chain.node.ConsensusThreshold(timestamp)
	return chain.node.CacheVerifyCosi(s.Hash, s.Signature, publics, base)
}

func (chain *Chain) ConsensusKeys(round, timestamp uint64) []*crypto.Key {
	publics := chain.node.ConsensusKeys(timestamp)
	if chain.IsPledging() && round == 0 {
		publics = append(publics, &chain.ConsensusInfo.Signer.PublicSpendKey)
	}
	return publics
}

func (chain *Chain) legacyVerifyFinalization(timestamp uint64, sigs []*crypto.Signature) bool {
	return len(sigs) >= chain.node.ConsensusThreshold(timestamp)
}
