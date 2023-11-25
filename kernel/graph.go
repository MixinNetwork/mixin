package kernel

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
)

const (
	GraphOperationClassAtomic       = 0x00
	GraphOperationClassNormalLedger = 0x01
)

func (chain *Chain) startNewRoundAndPersist(cache *CacheRound, references *common.RoundLink, timestamp uint64, finalized bool) (*CacheRound, *FinalRound, bool, error) {
	dummyExternal := cache.References.External
	final, dummy, err := chain.validateNewRound(cache, references, timestamp, finalized)
	if err != nil {
		return nil, nil, false, err
	} else if final == nil {
		return nil, nil, false, nil
	}
	cache = &CacheRound{
		NodeId:     chain.ChainId,
		Number:     final.Number + 1,
		Timestamp:  timestamp,
		References: references.Copy(),
	}
	if dummy {
		cache.References.External = dummyExternal
	}

	err = chain.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
	if err != nil {
		panic(err)
	}
	chain.assignNewGraphRound(final, cache)
	return cache, final, dummy, nil
}

func (chain *Chain) validateNewRound(cache *CacheRound, references *common.RoundLink, timestamp uint64, finalized bool) (*FinalRound, bool, error) {
	if chain.ChainId != cache.NodeId {
		panic("should never be here")
	}
	final := cache.asFinal()
	if final == nil {
		return nil, false, fmt.Errorf("self cache snapshots not collected yet %s %d", chain.ChainId, cache.Number)
	}
	if references.Self != final.Hash {
		return nil, false, fmt.Errorf("self cache snapshots not match yet %s %s", chain.ChainId, references.Self)
	}

	external, err := chain.persistStore.ReadRound(references.External)
	if err != nil {
		return nil, false, err
	}
	if external == nil && finalized {
		return final, true, nil
	}
	if external == nil {
		return nil, false, fmt.Errorf("external round %s not collected yet", references.External)
	}
	if !external.Hash.HasValue() || external.Hash != references.External {
		panic(references.External)
	}
	err = chain.updateExternal(final, external, timestamp, !finalized)
	if err != nil {
		return nil, false, err
	}

	return final, false, nil
}

func (chain *Chain) updateEmptyHeadRoundAndPersist(m *CosiAction, final *FinalRound, cache *CacheRound, references *common.RoundLink, timestamp uint64, strict bool) error {
	if len(cache.Snapshots) != 0 {
		return fmt.Errorf("malformated head round references not empty")
	}
	if references.Self != cache.References.Self {
		return fmt.Errorf("malformated head round references self diff %s %s", references.Self, cache.References.Self)
	}
	external, err := chain.persistStore.ReadRound(references.External)
	if err != nil || external == nil {
		return fmt.Errorf("round references external not ready yet %v %v", external, err)
	}
	if !external.Hash.HasValue() || external.Hash != references.External {
		panic(references.External)
	}

	err = chain.updateExternal(final, external, timestamp, strict)
	if err != nil {
		return err
	}

	cache.References = references.Copy()
	err = chain.persistStore.UpdateEmptyHeadRound(cache.NodeId, cache.Number, cache.References)
	if err != nil {
		panic(err)
	}
	chain.assignNewGraphRound(final, cache)
	return nil
}

func (chain *Chain) updateExternal(final *FinalRound, external *common.Round, roundTime uint64, strict bool) error {
	if final.NodeId == external.NodeId {
		return fmt.Errorf("external reference self %s", final.NodeId)
	}
	if external.Number < chain.State.RoundLinks[external.NodeId] {
		return fmt.Errorf("external reference back link %d %d",
			external.Number, chain.State.RoundLinks[external.NodeId])
	}
	link, err := chain.persistStore.ReadLink(final.NodeId, external.NodeId)
	if err != nil {
		return err
	}
	if link != chain.State.RoundLinks[external.NodeId] {
		panic(fmt.Errorf("should never be here %s=>%s %d %d",
			chain.ChainId, external.NodeId, link, chain.State.RoundLinks[external.NodeId]))
	}

	if strict {
		ec := chain.node.getOrCreateChain(external.NodeId)
		err := chain.checkReferenceSanity(ec, external, roundTime)
		if err != nil {
			return fmt.Errorf("external reference sanity %s", err)
		}
		threshold := external.Timestamp + config.SnapshotSyncRoundThreshold*config.SnapshotRoundGap*64
		best := chain.determineBestRound(roundTime)
		if best != nil && threshold < best.Start {
			return fmt.Errorf("external reference %v too early %v %f",
				*external, *best, time.Duration(best.Start-threshold).Seconds())
		}
	}

	chain.State.RoundLinks[external.NodeId] = external.Number
	return nil
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
	if n := rounds[len(rounds)-1].Number; n == final.Number {
		logger.Debugf("graph skip round %s %s %d\n", chain.node.IdForNetwork, chain.ChainId, final.Number)
		return
	} else if n+1 != final.Number {
		panic(fmt.Errorf("should never be here %s %d %d", final.NodeId, final.Number, n))
	}

	chain.StepForward()
	rounds = append(rounds, final.Copy())
	chain.State.RoundHistory = reduceHistory(rounds)
}

func reduceHistory(rounds []*FinalRound) []*FinalRound {
	last := rounds[len(rounds)-1]
	threshold := config.SnapshotReferenceThreshold * config.SnapshotRoundGap * 64
	if rounds[0].Start+threshold > last.Start && len(rounds) <= config.SnapshotReferenceThreshold {
		return rounds
	}
	newRounds := make([]*FinalRound, 0)
	for _, r := range rounds {
		if r.Start+threshold <= last.Start {
			continue
		}
		newRounds = append(newRounds, r)
	}
	if rc := len(newRounds) - config.SnapshotReferenceThreshold; rc > 0 {
		newRounds = newRounds[rc:]
	}
	return newRounds
}

// the external round to be referenced should be as latest as possible,
// this makes the graph stable. and to make the graph grow faster, it's
// recommended to reference a round about ten minutes agao.
//
// it's also important to reference the first accepted node round even if
// it's not consensus ready yet, because it's part of the graph.
func (chain *Chain) determineBestRound(roundTime uint64) *FinalRound {
	chain.node.chains.RLock()
	defer chain.node.chains.RUnlock()

	if chain.State == nil {
		return nil
	}

	var best *FinalRound
	var start, height uint64
	nodes := chain.node.NodesListWithoutState(roundTime, true)
	for _, cn := range nodes {
		id := cn.IdForNetwork
		if id == chain.ChainId {
			continue
		}

		ec, link := chain.node.chains.m[id], chain.State.RoundLinks[id]
		history := historySinceRound(ec.State.RoundHistory, link)
		if len(history) == 0 {
			continue
		}

		err := chain.checkReferenceSanity(ec, history[0].Common(), roundTime)
		if err != nil {
			continue
		}

		rts, rh := history[0].Start, uint64(len(history))
		if rh > height || rts > start {
			best, start, height = history[0], rts, rh
		}
	}

	return best
}

func (chain *Chain) checkReferenceSanity(ec *Chain, external *common.Round, roundTime uint64) error {
	if external.Timestamp > roundTime {
		return fmt.Errorf("external reference later than snapshot time %f",
			time.Duration(external.Timestamp-roundTime).Seconds())
	}
	if !chain.node.genesisNodesMap[external.NodeId] && external.Number < 7+config.SnapshotReferenceThreshold {
		return fmt.Errorf("external hint round too early yet not genesis %d",
			external.Number)
	}

	cr, fr := ec.State.CacheRound, ec.State.FinalRound
	if now := uint64(clock.Now().UnixNano()); fr.Start > now {
		return fmt.Errorf("external hint round timestamp too future %d %d",
			fr.Start, clock.Now().UnixNano())
	}
	if len(cr.Snapshots) == 0 && cr.Number == external.Number+1 && external.Number > 0 {
		return fmt.Errorf("external hint round without extra final yet %d",
			external.Number)
	}
	return nil
}

func historySinceRound(history []*FinalRound, link uint64) []*FinalRound {
	for i, r := range history {
		if r.Number >= link {
			return history[i:]
		}
	}
	return nil
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

func (node *Node) CacheVerifyCosi(snap crypto.Hash, sig *crypto.CosiSignature, cids []crypto.Hash, publics []*crypto.Key, threshold int) ([]crypto.Hash, bool) {
	key := sig.Signature[:]
	key = append(snap[:], key...)
	for _, pub := range publics {
		key = append(key, pub[:]...)
	}
	key = binary.BigEndian.AppendUint64(key, uint64(threshold))
	key = binary.BigEndian.AppendUint64(key, sig.Mask)
	value, found := node.cacheStore.Get(key)
	if found {
		signers := convertBytesToSigners(sig, value.([]byte))
		return signers, len(signers) == len(sig.Keys())
	}

	err := sig.FullVerify(publics, threshold, snap)
	if err != nil {
		logger.Verbosef("CacheVerifyCosi(%s, %d, %d) ERROR %s\n", snap, len(publics), threshold, err.Error())
		node.cacheStore.Set(key, []byte{0}, 1)
		return nil, false
	}

	signers := make([]crypto.Hash, len(sig.Keys()))
	for i, k := range sig.Keys() {
		signers[i] = cids[k]
	}
	vb := convertSignersToBytes(signers)
	node.cacheStore.Set(key, vb, int64(len(vb)))
	return signers, true
}

func convertBytesToSigners(sig *crypto.CosiSignature, b []byte) []crypto.Hash {
	if len(b) != len(sig.Keys())*len(crypto.Hash{}) {
		return nil
	}
	signers := make([]crypto.Hash, len(sig.Keys()))
	for i := 0; i < len(signers); i++ {
		var h crypto.Hash
		copy(h[:], b[i*32:i*32+32])
		signers[i] = h
	}
	return signers
}

func convertSignersToBytes(signers []crypto.Hash) []byte {
	var b []byte
	for _, h := range signers {
		b = append(b, h[:]...)
	}
	return b
}

func (chain *Chain) ConsensusKeys(round, timestamp uint64) ([]crypto.Hash, []*crypto.Key) {
	var signers []crypto.Hash
	var publics []*crypto.Key
	nodes := chain.node.NodesListWithoutState(timestamp, false)
	for _, cn := range nodes {
		if chain.node.ConsensusReady(cn, timestamp) {
			signers = append(signers, cn.IdForNetwork)
			publics = append(publics, &cn.Signer.PublicSpendKey)
		}
	}
	if chain.IsPledging() && round == 0 {
		signers = append(signers, chain.ChainId)
		publics = append(publics, &chain.ConsensusInfo.Signer.PublicSpendKey)
	}
	return signers, publics
}

func (chain *Chain) verifyFinalization(s *common.Snapshot) ([]crypto.Hash, bool) {
	switch s.Version {
	case common.SnapshotVersionCommonEncoding:
	default:
		return nil, false
	}

	if s.Signature == nil { // only genesis transaction can be valid at this situation
		return nil, false
	}

	timestamp := s.Timestamp
	if s.Hash.String() == mainnetNodeRemovalHackSnapshotHash {
		timestamp = timestamp - uint64(time.Minute)
	}

	cids, publics := chain.ConsensusKeys(s.RoundNumber, timestamp)
	base := chain.node.ConsensusThreshold(timestamp, true)
	return chain.node.CacheVerifyCosi(s.Hash, s.Signature, cids, publics, base)
}
