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

	err = tx.Validate(node.persistStore, false)
	if err != nil {
		return "", err
	}
	err = node.persistStore.CachePutTransaction(tx)
	if err != nil {
		return "", err
	}
	s := &common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: tx.PayloadHash(),
	}
	err = node.chain.AppendSelfEmpty(s)
	return tx.PayloadHash().String(), err
}

func (node *Node) LoopCacheQueue() error {
	defer close(node.cqc)

	offset, limit := crypto.Hash{}, 100
	for {
		period := time.Duration(config.SnapshotRoundGap)
		if offset.HasValue() {
			period = time.Millisecond * 300
		}
		timer := time.NewTimer(period)
		select {
		case <-node.done:
			return nil
		case <-timer.C:
		}
		caches, finals, _ := node.QueueState()
		if caches > 1000 || finals > 500 {
			timer.Stop()
			continue
		}

		neighbors := node.Peer.Neighbors()
		if len(neighbors) <= 0 {
			continue
		}
		var stale []crypto.Hash
		txs, err := node.persistStore.CacheListTransactions(offset, limit)
		for _, tx := range txs {
			offset = tx.PayloadHash()
			_, finalized, err := node.persistStore.ReadTransaction(offset)
			if err != nil {
				logger.Printf("LoopCacheQueue ReadTransaction ERROR %s %s\n", offset, err)
				continue
			}
			if len(finalized) > 0 {
				stale = append(stale, offset)
				continue
			}
			err = tx.Validate(node.persistStore, false)
			if err != nil {
				logger.Debugf("LoopCacheQueue Validate ERROR %s %s\n", offset, err)
				// FIXME not mark invalid tx as stale is to ensure final graph sync
				// but we need some way to mitigate cache transaction DoS attach from nodes
				continue
			}
			nbor := neighbors[rand.Intn(len(neighbors))]
			node.SendTransactionToPeer(nbor.IdForNetwork, offset)
			s := &common.Snapshot{
				Version:     common.SnapshotVersion,
				NodeId:      node.IdForNetwork,
				Transaction: tx.PayloadHash(),
			}
			node.chain.AppendSelfEmpty(s)
		}
		if len(txs) < limit {
			offset = crypto.Hash{}
		}
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

func (node *Node) QueueState() (uint64, uint64, map[string][2]uint64) {
	node.chains.RLock()
	defer node.chains.RUnlock()

	var caches, finals uint64
	state := make(map[string][2]uint64)
	for _, chain := range node.chains.m {
		sa := [2]uint64{
			uint64(len(chain.CachePool)),
			uint64(len(chain.finalActionsRing)),
		}
		round := chain.FinalPool[chain.FinalIndex]
		if round != nil {
			sa[1] = sa[1] + uint64(round.Size)
		}
		caches = caches + sa[0]
		finals = finals + sa[1]
		state[chain.ChainId.String()] = sa
	}
	return caches, finals, state
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
