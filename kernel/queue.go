package kernel

import (
	"encoding/binary"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/p2p"
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
		return old.PayloadHash().String(), node.persistStore.CacheQueueTransaction(tx)
	}

	err = tx.Validate(node.persistStore, clock.NowUnixNano(), false)
	if err != nil {
		return "", err
	}
	err = node.persistStore.CacheQueueTransaction(tx)
	if err != nil {
		return "", err
	}
	node.wakeCacheQueue()
	return tx.PayloadHash().String(), nil
}

func (node *Node) loopCacheQueue() {
	defer close(node.cqc)

	for {
		if node.waitCacheQueue(time.Duration(config.SnapshotRoundGap / 3)) {
			return
		}
		// A short debounce preserves batching while avoiding the old multi-second
		// delay for an otherwise idle queue.
		if node.waitOrDone(300 * time.Millisecond) {
			return
		}
		for node.popAndProcessCacheQueue() == common.SnapshotTransactionsMaximum {
		}
	}
}

// wakeCacheQueue signals loopCacheQueue that new transaction is available.
// The channel has capacity one, so repeated signals coalesce into a single
// immediate iteration instead of queueing up.
func (node *Node) wakeCacheQueue() {
	select {
	case node.queueWake <- struct{}{}:
	default:
	}
}

func (node *Node) waitCacheQueue(wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-node.done:
		return true
	case <-node.queueWake:
		return false
	case <-timer.C:
		return false
	}
}

func (node *Node) popAndProcessCacheQueue() int {
	caches, finals, _ := node.QueueState()
	if caches > 1000 || finals > 500 {
		logger.Printf("LoopCacheQueue QueueState too big %d %d\n", caches, finals)
		return 0
	}

	allNodes := node.ListWorkingAcceptedNodes(clock.NowUnixNano())
	if len(allNodes) <= 0 {
		return 0
	}

	txs, err := node.persistStore.CacheRetrieveTransactions(common.SnapshotTransactionsMaximum)
	if err != nil {
		logger.Printf("LoopCacheQueue CacheRetrieveTransactions ERROR %s\n", err)
		return 0
	}

	now := clock.Now()
	leadingNodes, leadingFilter := node.filterLeadingNodes(allNodes)

	filter := make(map[crypto.Hash]bool)
	var batchSize int
	var stale, batch []crypto.Hash
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
		batchSize += tx.ValidatedSize()
		if tx.IsSnapshotBatchable() && batchSize < p2p.TransportMessageMaxSize*2/3 {
			batch = append(batch, hash)
			continue
		}
		nbors := node.findSnapshotNodes(allNodes, leadingNodes, leadingFilter, now, hash)
		for _, nbor := range nbors {
			node.sendTransactionsToNode([]crypto.Hash{hash}, nbor)
		}
	}
	if len(batch) > 0 {
		if node.chainCanProposeSnapshot(allNodes, node.chain, uint64(now.UnixNano())) {
			// if a node can propose, then it will not send the transactions to others
			// because announcement and challenge transactions won't enter this node queue
			// so won't cause redundant snapshot for those transactions.
			// now the remaining issues are transactions sent before finalization snapshot
			// they entered the queue again
			node.sendTransactionsToNode(batch, node.IdForNetwork)
		} else {
			nbors := node.findSnapshotNodes(allNodes, leadingNodes, leadingFilter, now, batch[0])
			for _, nbor := range nbors {
				node.sendTransactionsToNode(batch, nbor)
			}
		}
	}
	err = node.persistStore.CacheRemoveTransactions(stale)
	if err != nil {
		logger.Printf("LoopCacheQueue CacheRemoveTransactions ERROR %s\n", err)
	}
	return len(txs)
}

func (node *Node) sendTransactionsToNode(txs []crypto.Hash, nbor crypto.Hash) {
	if nbor != node.IdForNetwork {
		err := node.SendTransactionsToPeer(nbor, txs, false)
		logger.Debugf("queue.SendTransactionsToPeer(%d, %s) => %v", len(txs), nbor, err)
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

func (node *Node) requeueTransactions(hashes []crypto.Hash) {
	requeued := false
	for _, hash := range hashes {
		tx, finalized, err := node.persistStore.ReadTransaction(hash)
		if err != nil {
			logger.Debugf("queue.requeueTransactions ReadTransaction(%s) => %v", hash, err)
			continue
		}
		if len(finalized) > 0 {
			continue
		}
		if tx == nil {
			tx, err = node.persistStore.CacheGetTransaction(hash)
			if err != nil {
				logger.Debugf("queue.requeueTransactions CacheGetTransaction(%s) => %v", hash, err)
				continue
			}
		}
		if tx == nil {
			continue
		}
		err = node.persistStore.CacheQueueTransaction(tx)
		if err != nil {
			logger.Debugf("queue.requeueTransactions CacheRequeueTransaction(%s) => %v", hash, err)
		} else {
			requeued = true
		}
	}
	if requeued {
		node.wakeCacheQueue()
	}
}

func (node *Node) canBatchSelfTransactions() bool {
	if node.chain == nil || node.chain.State == nil || node.chain.State.CacheRound == nil {
		return false
	}
	return node.chain.State.CacheRound.Number > 0
}

func (node *Node) canProposeSnapshot(all []*CNode) bool {
	if !node.canBatchSelfTransactions() {
		return false
	}
	for _, cn := range all {
		if cn.IdForNetwork == node.IdForNetwork {
			return node.CheckBroadcastedToPeers() && node.CheckCatchUpWithPeers()
		}
	}
	return false
}

func (node *Node) findSnapshotNodes(all, leading []*CNode, filter map[crypto.Hash]bool, now time.Time, hash crypto.Hash) []crypto.Hash {
	timestamp := uint64(now.UnixNano())
	ready := make([]crypto.Hash, 0, len(all))
	for _, cn := range all {
		chain := node.getChain(cn.IdForNetwork)
		local := cn.IdForNetwork == node.IdForNetwork
		if local {
			chain = node.chain
		}
		if node.chainCanProposeSnapshot(all, chain, timestamp) {
			ready = append(ready, cn.IdForNetwork)
		}
	}
	if len(ready) > 0 {
		return []crypto.Hash{ready[cacheQueueIndex(hash, now, len(ready))]}
	}
	return node.findRandomHeadNodeWithPossibleTail(all, leading, filter, now)
}

func (node *Node) chainCanProposeSnapshot(all []*CNode, chain *Chain, timestamp uint64) bool {
	if chain == nil {
		return false
	}
	local := node.IdForNetwork == chain.ChainId
	if local && !node.canProposeSnapshot(all) {
		return false
	}

	chain.RLock()
	if chain.State == nil || chain.State.CacheRound == nil || chain.State.CacheRound.Number == 0 {
		chain.RUnlock()
		return false
	}
	cache := chain.State.CacheRound
	cacheTimestamp := cache.Timestamp
	snapshots := append([]*common.Snapshot(nil), cache.Snapshots...)
	chain.RUnlock()

	if timestamp <= cacheTimestamp {
		return false
	}
	if len(snapshots) == 0 {
		return true
	}

	start := snapshots[0].Timestamp
	for _, snapshot := range snapshots[1:] {
		if snapshot.Timestamp < start {
			start = snapshot.Timestamp
		}
	}
	if timestamp < start+config.SnapshotRoundGap {
		return timestamp <= start+config.SnapshotRoundGap*4/5 && timestamp/OneDay == start/OneDay
	}
	return chain.determineBestRound(timestamp) != nil
}

func cacheQueueIndex(hash crypto.Hash, now time.Time, size int) int {
	bucket := uint64(now.UnixNano()) / config.SnapshotRoundGap
	seed := binary.BigEndian.Uint64(hash[:8])
	return int((seed + bucket) % uint64(size))
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

// TODO need to think whether I need to group transactions by hash and time to different nodes
func (node *Node) findRandomHeadNodeWithPossibleTail(all, leading []*CNode, filter map[crypto.Hash]bool, now time.Time) []crypto.Hash {
	idx := now.UnixNano() / int64(time.Minute) % int64(len(all))
	id := all[idx].IdForNetwork
	if filter[id] || len(leading) == 0 {
		return []crypto.Hash{id}
	}

	idx = now.UnixNano() / int64(time.Minute) % int64(len(leading))
	lid := leading[idx].IdForNetwork
	if lid == id {
		return []crypto.Hash{id}
	}
	logger.Debugf("findRandomHeadNodeWithPossibleTail(%d, %d) => %s %s", len(all), len(leading), id, lid)
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
