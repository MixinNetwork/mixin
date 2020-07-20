package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) checkFinalSnapshotTransaction(s *common.Snapshot) (*common.VersionedTransaction, error) {
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return nil, err
	}

	tx, _, err := node.persistStore.ReadTransaction(s.Transaction)
	if err == nil && tx != nil {
		err = node.validateKernelSnapshot(s, tx, true)
	}
	if err != nil || tx != nil {
		return tx, err
	}

	tx, err = node.persistStore.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return nil, err
	}
	err = node.validateKernelSnapshot(s, tx, true)
	if err != nil {
		return nil, err
	}

	err = tx.LockInputs(node.persistStore, true)
	if err != nil {
		return nil, err
	}
	return tx, node.persistStore.WriteTransaction(tx)
}

func (chain *Chain) tryToStartNewRound(s *common.Snapshot) (bool, error) {
	if chain.node.checkInitialAcceptSnapshotWeak(s) {
		return false, nil
	}
	if chain.State.CacheRound == nil {
		return false, fmt.Errorf("node not accepted yet %s %d %s", s.NodeId, s.RoundNumber, time.Unix(0, int64(s.Timestamp)).String())
	}

	cache := chain.State.CacheRound.Copy()
	final := chain.State.FinalRound.Copy()

	if s.RoundNumber != cache.Number+1 {
		return false, nil
	}

	dummyExternal := cache.References.External
	round, dummy, err := chain.startNewRound(s, cache, true)
	if err != nil {
		return false, err
	} else if round == nil {
		return false, nil
	} else {
		final = round
	}
	cache = &CacheRound{
		NodeId:    s.NodeId,
		Number:    s.RoundNumber,
		Timestamp: s.Timestamp,
		References: &common.RoundLink{
			Self:     s.References.Self,
			External: s.References.External,
		},
	}
	if dummy {
		cache.References.External = dummyExternal
	}
	err = chain.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
	if err != nil {
		panic(err)
	}

	chain.assignNewGraphRound(final, cache)
	return dummy, nil
}

func (chain *Chain) legacyAppendFinalization(peerId crypto.Hash, s *common.Snapshot) error {
	s.Hash = s.PayloadHash()
	if !chain.node.legacyVerifyFinalization(s.Timestamp, s.Signatures) {
		return nil
	}

	sigs := make([]*crypto.Signature, 0)
	signaturesFilter := make(map[string]bool)
	signersMap := make(map[crypto.Hash]bool)
	for _, sig := range s.Signatures {
		if signaturesFilter[sig.String()] {
			continue
		}
		for idForNetwork, cn := range chain.node.ConsensusNodes {
			if signersMap[idForNetwork] {
				continue
			}
			if chain.node.CacheVerify(s.Hash, *sig, cn.Signer.PublicSpendKey) {
				sigs = append(sigs, sig)
				signersMap[idForNetwork] = true
				break
			}
		}
		if n := chain.node.ConsensusPledging; n != nil {
			id := n.IdForNetwork(chain.node.networkId)
			if id == s.NodeId && s.RoundNumber == 0 && chain.node.CacheVerify(s.Hash, *sig, n.Signer.PublicSpendKey) {
				sigs = append(sigs, sig)
				signersMap[id] = true
			}
		}
		signaturesFilter[sig.String()] = true
	}
	s.Signatures = sigs

	if !chain.node.legacyVerifyFinalization(s.Timestamp, s.Signatures) {
		return nil
	}

	return chain.QueueAppendSnapshot(peerId, s, true)
}
