package kernel

import (
	"math/big"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
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
		return old.PayloadHash().String(), node.persistStore.CachePutTransaction(tx)
	}

	err = tx.Validate(node.persistStore, clock.NowUnixNano(), false)
	if err != nil {
		return "", err
	}
	err = node.persistStore.CachePutTransaction(tx)
	if err != nil {
		return "", err
	}
	return tx.PayloadHash().String(), nil
}

func (node *Node) loopCacheQueue() {
	defer close(node.cqc)

	for !node.waitOrDone(time.Duration(config.SnapshotRoundGap)) {
		caches, finals, _ := node.QueueState()
		if caches > 1000 || finals > 500 {
			logger.Printf("LoopCacheQueue QueueState too big %d %d\n", caches, finals)
			continue
		}

		allNodes := node.ListWorkingAcceptedNodes(clock.NowUnixNano())
		if len(allNodes) <= 0 {
			continue
		}

		txs, err := node.persistStore.CacheRetrieveTransactions(100)
		if err != nil {
			logger.Printf("LoopCacheQueue CacheRetrieveTransactions ERROR %s\n", err)
			continue
		}

		now := clock.Now()
		filter := make(map[crypto.Hash]bool)
		var stale, batch, single []crypto.Hash
		leadingNodes, leadingFilter := node.filterLeadingNodes(allNodes)
		for _, tx := range txs {
			hash := tx.PayloadHash()
			if filter[hash] {
				continue
			}
			filter[hash] = true
			_, finalized, err := node.persistStore.ReadTransaction(hash)
			if err != nil {
				logger.Printf("LoopCacheQueue ReadTransaction ERROR %s %s\n", hash, err)
				continue
			}
			if len(finalized) > 0 {
				stale = append(stale, hash)
				continue
			}
			err = tx.Validate(node.persistStore, uint64(now.UnixNano()), false)
			if err != nil {
				logger.Debugf("LoopCacheQueue Validate ERROR %s %s\n", hash, err)
				// FIXME not mark invalid tx as stale is to ensure final graph sync
				// but we need some way to mitigate cache transaction DoS attack from nodes
				continue
			}
			nbor := node.electSnapshotNode(tx.TransactionType(), uint64(now.UnixNano()))
			if nbor.HasValue() {
				node.sendTransactionsToNode([]crypto.Hash{hash}, nbor)
				continue
			}
			if tx.IsSnapshotBatchable() {
				batch = append(batch, hash)
			} else {
				single = []crypto.Hash{hash}
				break
			}
		}
		if len(batch) > 0 {
			nbors := node.findRandomHeadNodeWithPossibleTail(allNodes, leadingNodes, leadingFilter, now, batch[0])
			for _, nbor := range nbors {
				node.sendTransactionsToNode(batch, nbor)
			}
		}
		if len(single) > 0 {
			nbors := node.findRandomHeadNodeWithPossibleTail(allNodes, leadingNodes, leadingFilter, now, single[0])
			for _, nbor := range nbors {
				node.sendTransactionsToNode(single, nbor)
			}
		}
		err = node.persistStore.CacheRemoveTransactions(stale)
		if err != nil {
			logger.Printf("LoopCacheQueue CacheRemoveTransactions ERROR %s\n", err)
		}
	}
}

func (node *Node) sendTransactionsToNode(txs []crypto.Hash, nbor crypto.Hash) {
	if nbor != node.IdForNetwork {
		for _, hash := range txs {
			err := node.SendTransactionToPeer(nbor, hash)
			logger.Debugf("queue.SendTransactionToPeer(%s, %s) => %v", hash, nbor, err)
		}
	} else if !node.canBatchSelfTransactions() {
		for _, hash := range txs {
			s := &common.Snapshot{
				Version: common.SnapshotVersionCommonEncoding,
				NodeId:  node.IdForNetwork,
			}
			s.AddTransaction(hash)
			err := node.chain.AppendSelfEmpty(s)
			logger.Debugf("queue.AppendSelfEmpty(%v) => %v", s, err)
		}
	} else {
		s := &common.Snapshot{
			Version: common.SnapshotVersionCommonEncoding,
			NodeId:  node.IdForNetwork,
		}
		for _, hash := range txs {
			s.AddTransaction(hash)
		}
		err := node.chain.AppendSelfEmpty(s)
		logger.Debugf("queue.AppendSelfEmpty(%v) => %v", s, err)
	}
}

func (node *Node) canBatchSelfTransactions() bool {
	if node.chain == nil || node.chain.State == nil || node.chain.State.FinalRound == nil {
		return false
	}
	return node.chain.State.FinalRound.Number > 0
}

func (node *Node) filterLeadingNodes(all []*CNode) ([]*CNode, map[crypto.Hash]bool) {
	node.chains.RLock()
	defer node.chains.RUnlock()

	threshold := 5 * uint64(time.Minute)
	now := clock.NowUnixNano()

	leading := make([]*CNode, 0)
	filter := make(map[crypto.Hash]bool)
	for _, cn := range all {
		chain := node.chain.node.chains.m[cn.IdForNetwork]
		if chain.State == nil {
			continue
		}
		f := chain.State.FinalRound
		if f.Start+threshold < now {
			continue
		}
		leading = append(leading, cn)
		filter[cn.IdForNetwork] = true
	}
	return leading, filter
}

func (node *Node) findRandomHeadNodeWithPossibleTail(all, leading []*CNode, filter map[crypto.Hash]bool, now time.Time, hash crypto.Hash) []crypto.Hash {
	hb := new(big.Int).SetBytes(hash[:])
	mb := big.NewInt(now.UnixNano() / int64(time.Minute))
	ib := new(big.Int).Add(hb, mb)
	idx := new(big.Int).Mod(ib, big.NewInt(int64(len(all)))).Int64()
	id := all[idx].IdForNetwork
	if filter[id] || len(leading) == 0 {
		return []crypto.Hash{id}
	}

	idx = new(big.Int).Mod(ib, big.NewInt(int64(len(leading)))).Int64()
	lid := leading[idx].IdForNetwork
	logger.Debugf("findRandomHeadNodeWithPossibleTail(%s, %d, %d) => %s %s", hash, len(all), len(leading), id, lid)
	return []crypto.Hash{id, lid}
}

func (node *Node) QueueState() (uint64, uint64, map[string][2]uint64) {
	node.chains.RLock()
	defer node.chains.RUnlock()

	var caches, finals uint64
	state := make(map[string][2]uint64)
	accepted := node.NodesListWithoutState(clock.NowUnixNano(), true)
	for _, cn := range accepted {
		chain := node.chains.m[cn.IdForNetwork]
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
	ns := &common.Snapshot{
		Version: common.SnapshotVersionCommonEncoding,
		NodeId:  s.NodeId,
	}
	for _, tx := range s.Transactions {
		ns.AddTransaction(tx)
	}
	return chain.AppendSelfEmpty(ns)
}
