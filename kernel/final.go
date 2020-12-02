package kernel

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) checkTxInStorage(id crypto.Hash) (*common.VersionedTransaction, error) {
	tx, _, err := node.persistStore.ReadTransaction(id)
	if err != nil || tx != nil {
		return tx, err
	}

	return node.persistStore.CacheGetTransaction(id)
}

func (chain *Chain) legacyAppendFinalization(peerId crypto.Hash, s *common.Snapshot) error {
	if chain.ChainId != s.NodeId {
		panic("should never be here")
	}

	sigs := make([]*crypto.Signature, 0)
	signaturesFilter := make(map[string]bool)
	signersMap := make(map[crypto.Hash]bool)
	for _, sig := range s.Signatures {
		if signaturesFilter[sig.String()] {
			continue
		}
		nodes := chain.node.NodesListWithoutState(s.Timestamp, true)
		for _, cn := range nodes {
			if signersMap[cn.IdForNetwork] {
				continue
			}
			if chain.node.CacheVerify(s.Hash, *sig, cn.Signer.PublicSpendKey) {
				sigs = append(sigs, sig)
				signersMap[cn.IdForNetwork] = true
				break
			}
		}

		if chain.IsPledging() && s.RoundNumber == 0 && chain.node.CacheVerify(s.Hash, *sig, chain.ConsensusInfo.Signer.PublicSpendKey) {
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
