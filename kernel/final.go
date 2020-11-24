package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) checkTxInStorage(id crypto.Hash) (bool, error) {
	tx, _, err := node.persistStore.ReadTransaction(id)
	if err != nil || tx != nil {
		return tx != nil, err
	}

	tx, err = node.persistStore.CacheGetTransaction(id)
	return tx != nil, err
}

func (node *Node) checkFinalSnapshotTransaction(s *common.Snapshot) (*common.VersionedTransaction, bool, error) {
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return nil, inNode, err
	}

	tx, _, err := node.persistStore.ReadTransaction(s.Transaction)
	if err == nil && tx != nil {
		err = node.validateKernelSnapshot(s, tx, true)
	}
	if err != nil || tx != nil {
		return tx, false, err
	}

	tx, err = node.persistStore.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return nil, false, err
	}
	err = node.validateKernelSnapshot(s, tx, true)
	if err != nil {
		return nil, false, err
	}

	err = tx.LockInputs(node.persistStore, true)
	if err != nil {
		return nil, false, err
	}
	return tx, false, node.persistStore.WriteTransaction(tx)
}

func (chain *Chain) tryToStartNewRound(s *common.Snapshot) (bool, error) {
	if chain.ChainId != s.NodeId {
		panic("should never be here")
	}

	if chain.State.FinalRound == nil && s.RoundNumber == 0 {
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
	if chain.ChainId != s.NodeId {
		panic("should never be here")
	}

	if !chain.legacyVerifyFinalization(s.Timestamp, s.Signatures) {
		logger.Verbosef("ERROR legacyVerifyFinalization %s %v %d\n", peerId, s, chain.node.ConsensusThreshold(s.Timestamp))
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
		if chain.State.FinalRound == nil && s.RoundNumber == 0 && chain.node.CacheVerify(s.Hash, *sig, chain.ConsensusInfo.Signer.PublicSpendKey) {
			sigs = append(sigs, sig)
			signersMap[chain.ChainId] = true
		}
		signaturesFilter[sig.String()] = true
	}

	if len(sigs) != len(s.Signatures) {
		logger.Verbosef("ERROR legacyVerifyFinalization some node not accepted yet %s %v %d %d %d\n", peerId, s, chain.node.ConsensusThreshold(s.Timestamp), len(s.Signatures), len(sigs))
		return nil
	}
	s.Signatures = sigs

	if !chain.legacyVerifyFinalization(s.Timestamp, s.Signatures) {
		logger.Verbosef("ERROR RE legacyVerifyFinalization %s %v %d\n", peerId, s, chain.node.ConsensusThreshold(s.Timestamp))
		return nil
	}

	return chain.AppendFinalSnapshot(peerId, s)
}
