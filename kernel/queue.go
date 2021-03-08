package kernel

import (
	"math/rand"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) QueueTransaction(tx *common.VersionedTransaction) (string, error) {
	hash := tx.PayloadHash()
	_, finalized, err := node.persistStore.ReadTransaction(hash)
	if err != nil {
		return "", err
	}
	if len(finalized) > 0 {
		return hash.String(), nil
	}

	old, err := node.persistStore.CacheGetTransaction(hash)
	if err != nil {
		return "", err
	}
	if old != nil {
		return old.PayloadHash().String(), nil
	}

	err = tx.Validate(node.persistStore)
	if err != nil {
		return "", err
	}
	err = node.persistStore.CachePutTransaction(tx)
	if err != nil {
		return "", err
	}
	chain := node.GetOrCreateChain(node.IdForNetwork)
	s := &common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: tx.PayloadHash(),
	}
	err = chain.AppendSelfEmpty(s)
	return tx.PayloadHash().String(), err
}

func (node *Node) LoopCacheQueue() error {
	defer close(node.cqc)

	chain := node.GetOrCreateChain(node.IdForNetwork)

	for {
		timer := time.NewTimer(time.Duration(config.SnapshotRoundGap))
		select {
		case <-node.done:
			return nil
		case <-timer.C:
		}

		neighbors := node.Peer.Neighbors()
		var stale []crypto.Hash
		err := node.persistStore.CacheListTransactions(func(tx *common.VersionedTransaction) error {
			hash := tx.PayloadHash()
			_, finalized, err := node.persistStore.ReadTransaction(hash)
			if err != nil {
				logger.Printf("LoopCacheQueue ReadTransaction ERROR %s\n", err)
				return nil
			}
			if len(finalized) > 0 {
				stale = append(stale, hash)
				return nil
			}
			err = tx.Validate(node.persistStore)
			if err != nil {
				logger.Printf("LoopCacheQueue Validate ERROR %s\n", err)
				stale = append(stale, hash)
				return nil
			}
			peer := neighbors[rand.Intn(len(neighbors))]
			node.SendTransactionToPeer(peer.IdForNetwork, hash)
			s := &common.Snapshot{
				Version:     common.SnapshotVersion,
				NodeId:      node.IdForNetwork,
				Transaction: tx.PayloadHash(),
			}
			chain.AppendSelfEmpty(s)
			return nil
		})
		if err != nil {
			logger.Printf("LoopCacheQueue CacheListTransactions ERROR %s\n", err)
		}
		err = node.persistStore.CacheRemoveTransactions(stale)
		if err != nil {
			logger.Printf("LoopCacheQueue CacheRemoveTransactions ERROR %s\n", err)
		}

		timer.Stop()
	}
}

func (chain *Chain) clearAndQueueSnapshotOrPanic(s *common.Snapshot) error {
	if chain.ChainId != s.NodeId {
		panic("should never be here")
	}
	return chain.AppendSelfEmpty(&common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      s.NodeId,
		Transaction: s.Transaction,
	})
}
