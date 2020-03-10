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
	if err != nil || tx != nil {
		return tx, err
	}

	tx, err = node.persistStore.CacheGetTransaction(s.Transaction)
	if err != nil || tx == nil {
		return nil, err
	}

	err = tx.LockInputs(node.persistStore, true)
	if err != nil {
		return nil, err
	}
	return tx, node.persistStore.WriteTransaction(tx)
}

func (node *Node) tryToStartNewRound(s *common.Snapshot) (bool, error) {
	if node.checkInitialAcceptSnapshotWeak(s) {
		return false, nil
	}
	if node.Graph.CacheRound[s.NodeId] == nil {
		return false, fmt.Errorf("node not accepted yet %s %d %s", s.NodeId, s.RoundNumber, time.Unix(0, int64(s.Timestamp)).String())
	}

	cache := node.Graph.CacheRound[s.NodeId].Copy()
	final := node.Graph.FinalRound[s.NodeId].Copy()

	if s.RoundNumber != cache.Number+1 {
		return false, nil
	}

	dummyExternal := cache.References.External
	round, dummy, err := node.startNewRound(s, cache, true)
	if err != nil {
		return false, err
	} else if round == nil {
		return false, nil
	} else {
		final = round
	}
	cache = &CacheRound{
		NodeId:     s.NodeId,
		Number:     s.RoundNumber,
		Timestamp:  s.Timestamp,
		References: s.References,
	}
	if dummy {
		cache.References.External = dummyExternal
	}
	err = node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, final.Start)
	if err != nil {
		panic(err)
	}

	node.assignNewGraphRound(final, cache)
	return dummy, nil
}

func (node *Node) legacyAppendFinalization(peerId crypto.Hash, s *common.Snapshot) error {
	s.Hash = s.PayloadHash()
	if !node.legacyVerifyFinalization(s.Timestamp, s.Signatures) {
		return nil
	}

	node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash)
	err := node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash)
	if err != nil {
		return err
	}
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil || inNode {
		return err
	}

	sigs := make([]*crypto.Signature, 0)
	signaturesFilter := make(map[string]bool)
	signersMap := make(map[crypto.Hash]bool)
	for _, sig := range s.Signatures {
		if signaturesFilter[sig.String()] {
			continue
		}
		for idForNetwork, cn := range node.ConsensusNodes {
			if signersMap[idForNetwork] {
				continue
			}
			if node.CacheVerify(s.Hash, *sig, cn.Signer.PublicSpendKey) {
				sigs = append(sigs, sig)
				signersMap[idForNetwork] = true
				break
			}
		}
		if n := node.ConsensusPledging; n != nil {
			id := n.IdForNetwork(node.networkId)
			if id == s.NodeId && s.RoundNumber == 0 && node.CacheVerify(s.Hash, *sig, n.Signer.PublicSpendKey) {
				sigs = append(sigs, sig)
				signersMap[id] = true
			}
		}
		signaturesFilter[sig.String()] = true
	}
	s.Signatures = sigs

	if !node.legacyVerifyFinalization(s.Timestamp, s.Signatures) {
		return nil
	}

	return node.QueueAppendSnapshot(peerId, s, true)
}
