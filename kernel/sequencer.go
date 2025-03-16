package kernel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"slices"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/p2p"
)

// mainnet upgrade:
// 1. block chain production, no strict validations or slashings
// 2. remove node round, snapshot reference previous snap and block hash
// 3. sync changes, remove the old topo sync, just keep block sync
// 4. slashes enable

// The sequencer will make a blockchain with a total order in
// almost real time, according to the configured sequencing time,
// the sequencing time for the mainnet is at most 3 seconds.
//
// The accepted working nodes are a global total ordered list, by
// order, each node will add at most 512 snapshots to a block per
// the sequencing time, i.e. at 3 seconds for the mainnet. But
// the block size should not exceed 16MB.
//
// The sequencing time is simply defined by the blocktime. Each node
// will add a blocktime to the block it created. The blocktime
// must monotically increase, and it must never exceed the unix
// timestamp for more than 1 minute. If T nodes decide a blocktime
// incorrect, then the node will be slashed.
//
// The block has a strictly monotically increased block number. And
// every block will be broadcasted to all other nodes directly.
//
// The node receive a new block from the previous node in the nodes list,
// then it will check the sequencing time and local snapshots, then make
// a new block according to the rules. If a node doesn't have the previous
// blocks, then it must request all the blocks from the new block producer.
//
// The node can add any snapshots to the block as they want, but:
//
// 1. The snapshot hash must be globally unique in the whole blockchain,
//    otherwise the node will be slashed. NOPE, can be included in multi
//    blocks, just reduce the block score, thus reduce the reward
// 2. The snapshot must not be leaded by the node, the node could
//    only be a signer, or nothing, otherwise the node will be slashed.
//    This rule gives much higher chance that the snapshot is visible
//    to all other nodes. Slash with 100XIN
// 3. The snapshots must have the same concensus nodes list, i.e. they
//    are produced by the same batch of kernel nodes???? NO, still needs
//    to use the current graph timestamp as sequencer timestamp, because
//    the sequencer may have different nodes list than the cosi loop.
//    Like the mainnet has already run for a long time, most nodes are
//    removed already, then we can only produce blocks with current
//    nodes list. This is also efficient, because the sequencer loop
//    won't block the concurrent cosi loop.
// 3. The node will add an empty block if it has no snapshots available.
// 4. The node will check all the snapshots of the last non-empty block,
//    and all the snapshots should be available in the node itself
//    topology, and if not, the node will wait for 1 minute, then the node
//    will vote slash with the missing snapshots, and broadcast the vote
//    to the next node, then the next node will respond the snapshots if
//    the snapshots are available, or vote slash to the next node again.
//    once the vote members reach threshold, then the node will be slashed
//    for 13439XIN. And if the snapshots are all found before the threshold
//    is reached, then the block chain will resume from where it blocked.
//    During the process, some snapshots may be already found, so only
//    The missing snapshots will be delivered to the next node to vote.
//    NONONO
//    The procedure above is invalid, check voteTheBlockSlash
//
// The block producing order must be obeyed in time, if a node doesn't
// broacast its new block to the other nodes in 7 minutes, then all the
// nodes will start a voting process to slash the incompetent node. The
// time is determined by the previous block timestamp and the kernel
// graph timestamp. An attacker may produce a block every 6 minutes
// to make the blockchain slow, so we may have a slow node detection
// that if the node is always the slowest in the blockchain, and there
// are more than 7 blocks are more than 1 minute, then it will be slashed.
// And if it's just slow, the mint reward could be adjusted.
//
// The node produces a invalid block according to rules 1 and 2, then
// this block will still remain in the blockchain because it has all
// valid snapshots. But since the node will broadcast this invalid
// block to all other nodes, otherwise it will be slashed due to expiration.
// Then since all nodes received this invalid block, they will start a
// voting process to slash this malicious node.
//
// The node can only start producing a block when it's in its order.
// If the previous node is slashed due to expiration, then the nodes
// list will be reloaded due to the slash transaction. Then the block
// producing will start from the first node in the order. A node should
// never start a new block if its previous node is waiting.
//
// The whole blockchain should be considered final at any time, because
// we ensure all the snapshots are finalized correctly.
//
// The sequencer is also the slashing commander, for every 512 blocks
// the block producer must produce a checkpoint snapshot that has the
// all 512 blocks hash included, thus ensures the snapshot topology
// and the block chain is cross-referenced. While producing blocks,
// the sequencer will vote on all slashing rules, not only those one
// written here for block producing slashes.

const (
	BlockProducerTimeoutDuration = time.Minute
	BlockNumberEmpty             = ^uint64(0)

	// including snapshots and transactions
	BlockMaximumSize = 4 * common.ExtraSizeStorageCapacity
)

type Sequencer struct {
	CurrentBlock      *common.Block
	SequencedTopology uint64
	heads             map[crypto.Hash]*NodeHeadRequest

	incomingBlocks chan *common.BlockWithTransactions
	incomingHeads  chan *NodeHeadRequest
	node           *Node
}

type NodeHeadRequest struct {
	NodeId crypto.Hash
	Number uint64

	// how many blocks to request
	SyncRequest uint64

	// this is calculated by myself, we will prefer a neighbor to request
	// SyncRequest blocks? Or I MUST use a neighbor, because if my neighbor
	// doesn't have the blocks, the neighbor will request from its neighbors
	isNeighbor bool
	updatedAt  time.Time
}

func (node *Node) startSequencer() {
	slashingChecker := time.NewTicker(BlockProducerTimeoutDuration)
	defer slashingChecker.Stop()

	headerTicker := time.NewTicker(time.Second)
	defer headerTicker.Stop()

	blockTicker := time.NewTicker(time.Duration(config.SnapshotRoundGap))
	defer blockTicker.Stop()

	current, err := node.persistStore.ReadLastBlock()
	if err != nil {
		panic(err)
	}
	topo, err := node.persistStore.ReadSequencedTopology()
	if err != nil {
		panic(err)
	}
	seq := &Sequencer{
		CurrentBlock:      current,
		SequencedTopology: topo,
		incomingBlocks:    make(chan *common.BlockWithTransactions, 1024),
		incomingHeads:     make(chan *NodeHeadRequest, 8192),
		heads:             make(map[crypto.Hash]*NodeHeadRequest),
		node:              node,
	}
	node.sequencer = seq

	for {
		select {
		case h := <-seq.incomingHeads:
			last := seq.heads[h.NodeId]
			if last != nil && last.Number >= h.Number {
				continue
			}
			h.isNeighbor = seq.node.CheckNeighbor(h.NodeId)
			h.updatedAt = clock.Now()
			seq.heads[h.NodeId] = h
			seq.broadcastBlocksToNode(h.NodeId, h.Number, h.SyncRequest)
		case b := <-seq.incomingBlocks:
			err := seq.validateAndProcessIncomingBlock(b)
			if err != nil {
				panic(err)
			}
			seq.tryToProduceBlock()
			// always check synced for each step
			// the preivous node should broadcast precedent blocks too
			// I will start produce block from here if I receive a block from
			// the previous block, then it must be my order. I only produce
			// a block if I have any available snapshots, otherwise just skip
		case <-slashingChecker.C:
			seq.doSlashingVote()
		case <-headerTicker.C:
			// Broadcast my own block head to others, including all neighbors
			// and all consensus nodes. No need to check synced state.
			// here is the only place where I request more blocks from other nodes
			seq.broadcastHeader()
		case <-blockTicker.C:
			// I will start produce block from here if I have no blocks yet
			// from incomingBblocks channel. Here I will produce a block, or
			// empty block in any way
			logger.Debugf("sequencer.tryToProduceBlock(%s) on ticker", seq.node.IdForNetwork)
			seq.tryToProduceBlock()
		}
	}
}

func (seq *Sequencer) checkMyTurn() bool {
	if !seq.checkSynced() {
		// FIXME sometimes this is very slow for a node to become synced
		return false
	}
	next := seq.getNextProducer(seq.node.GraphTimestamp)
	logger.Debugf("sequenced.checkMyTurn(%s) => %s", seq.node.IdForNetwork, next.IdForNetwork)
	return next.IdForNetwork == seq.node.IdForNetwork
}

// THIS is most important, when pledging, removing, accepting
// how to fix this order thing?
// maybe for each nodes list change, we commit it to the snapshot?
// and we ensure all the nodes accept this change with signature
func (seq *Sequencer) getNextProducer(timestamp uint64) *CNode {
	producers := seq.node.GetBlockProducers(timestamp)
	if seq.CurrentBlock == nil {
		return producers[0]
	}
	now := slices.IndexFunc(producers, func(cn *CNode) bool {
		return cn.IdForNetwork == seq.CurrentBlock.NodeId
	})
	next := now + 1
	if next == len(producers) {
		next = 0
	}
	return producers[next]
}

func (seq *Sequencer) nextBlockNumber() uint64 {
	if seq.CurrentBlock == nil {
		return 0
	}
	return seq.CurrentBlock.Number + 1
}

func (seq *Sequencer) currentBlockTimestamp() uint64 {
	if seq.CurrentBlock == nil {
		return 0
	}
	return seq.CurrentBlock.Timestamp
}

// we never proactively send blocks to others, unless we are producing blocks
// otherwise we send blocks because they requested
func (seq *Sequencer) broadcastBlocksToNode(peerId crypto.Hash, height, count uint64) {
	logger.Printf("broadcastBlocksToNode(%s, %d, %d)", peerId, height, count)
	if count < 1 {
		return
	}
	if count > 512 {
		count = 512
	}
	var from uint64
	if height != BlockNumberEmpty {
		from = height + 1
	}
	var msgs [][]byte
	var totalSize int
	for i := uint64(0); i < count; i++ {
		block, err := seq.node.persistStore.ReadBlockWithTransactions(from + i)
		if err != nil {
			panic(err)
		}
		b := block.Marshal()
		if totalSize+len(b) >= p2p.TransportMessageMaxSize*9/10 {
			break
		}
		msgs = append(msgs, b)
		totalSize = totalSize + len(b)
	}
	err := seq.node.Peer.SendBlockSyncMessage(peerId, msgs)
	if err != nil {
		panic(err)
	}
}

func (seq *Sequencer) broadcastHeader() {
	nodes := seq.node.NodesListWithoutState(seq.node.GraphTimestamp, true)
	neighbors := seq.node.Peer.Neighbors()
	peers := make(map[crypto.Hash]bool)
	for _, cn := range nodes {
		peers[cn.IdForNetwork] = true
	}
	for _, p := range neighbors {
		peers[p.IdForNetwork] = true
	}
	// TODO block signature
	number := BlockNumberEmpty
	if b := seq.CurrentBlock; b != nil {
		number = b.Number
	}

	for id := range peers {
		var syncRequest uint64
		h := seq.heads[id]
		if h != nil && h.isNeighbor && h.Number != BlockNumberEmpty {
			if number == BlockNumberEmpty {
				syncRequest = h.Number
			} else if h.Number > number {
				syncRequest = h.Number - number
			}
		}
		if syncRequest >= 64 {
			syncRequest = 64
		}
		err := seq.node.Peer.SendBlockHeaderMessage(id, number, syncRequest)
		if err != nil {
			logger.Printf("SendBlockHeaderMessage(%s) => %v\n", id, err)
		}
	}
}

func (seq *Sequencer) tryToProduceBlock() {
	if !seq.checkMyTurn() {
		return
	}
	log.Printf("sequencer.tryToProduceBlock(%s) prepare", seq.node.IdForNetwork)
	b := &common.Block{
		NodeId:    seq.node.IdForNetwork,
		Timestamp: seq.node.GraphTimestamp,
	}
	if cur := seq.CurrentBlock; cur != nil {
		b.Number = cur.Number + 1
		b.Sequence = cur.Sequence + uint64(len(cur.Snapshots))
		b.Previous = cur.PayloadHash()
		if b.Timestamp <= cur.Timestamp {
			return
		}
	}
	candis, txs, err := seq.node.persistStore.ReadUnsequencedSnapshotsSinceTopology(seq.node.IdForNetwork, seq.SequencedTopology, 512)
	if err != nil {
		panic(err)
	}
	var totalSize int
	snaps := make(map[crypto.Hash]*common.Snapshot)
	for _, s := range candis {
		totalSize = totalSize + len(s.VersionedMarshal())
		for _, h := range s.Transactions {
			totalSize = totalSize + len(txs[h].PayloadMarshal())
		}
		if totalSize > BlockMaximumSize*7/8 {
			break
		}
		seq.SequencedTopology = s.TopologicalOrder
		if s.NodeId == seq.node.IdForNetwork {
			panic(s.TopologicalOrder)
		}
		b.Snapshots = append(b.Snapshots, s.PayloadHash())
		snaps[s.PayloadHash()] = s.Snapshot
	}
	sort.Slice(b.Snapshots, func(i, j int) bool {
		m := snaps[b.Snapshots[i]]
		n := snaps[b.Snapshots[j]]
		if m.Timestamp < n.Timestamp {
			return true
		}
		if m.Timestamp > n.Timestamp {
			return false
		}
		return bytes.Compare(b.Snapshots[i][:], b.Snapshots[j][:]) < 0
	})
	if len(b.Snapshots) > 0 {
		b.Timestamp = snaps[b.Snapshots[0]].Timestamp
	}
	b.Signature = seq.node.Signer.PrivateSpendKey.Sign(b.PayloadHash())
	err = seq.node.persistStore.WriteBlock(b, seq.SequencedTopology)
	log.Printf("sequencer.tryToProduceBlock(%s) write %v %d", seq.node.IdForNetwork, b, seq.SequencedTopology)
	if err != nil {
		panic(err)
	}
	seq.CurrentBlock = b

	nodes := seq.node.NodesListWithoutState(seq.node.GraphTimestamp, true)
	neighbors := seq.node.Peer.Neighbors()
	peers := make(map[crypto.Hash]bool)
	for _, cn := range nodes {
		peers[cn.IdForNetwork] = true
	}
	for _, p := range neighbors {
		peers[p.IdForNetwork] = true
	}
	msg := b.MarshalWithSnapshots(snaps, txs)
	for id := range peers {
		err = seq.node.Peer.SendBlockSyncMessage(id, [][]byte{msg})
		logger.Verbosef("SendBlockSyncMessage(%s, %v) => %v", id, b, err)
	}
}

func (seq *Sequencer) checkSynced() bool {
	if !seq.node.CheckCatchUpWithPeers() {
		return false
	}
	if !seq.node.CheckBroadcastedToPeers() {
		return false
	}
	timestamp, updated := seq.node.GraphTimestamp, 0
	threshold := seq.node.ConsensusThreshold(timestamp, false)
	for _, cn := range seq.node.GetBlockProducers(timestamp) {
		head := seq.heads[cn.IdForNetwork]
		if head == nil {
			continue
		}
		updated += 1
	}
	return updated >= threshold
	// check node synced
	// check sequencer heads synced
	// the node will check all other nodes, finds the highest number
	// then request to sync to that block number.
	// then check again, until it's fully synced then work on block producing
	// or expiration, slashing check.
	// all nodes could fully sync, because no nodes could produce next block
	// if the node doesn't fully sync and doesn't know who is next producer.
	//
	// a node will start produce, expiration or slashing check imediately asap
	// it's fully synced. means, it won't wait for other nodes to sync, no need
	// no responsibility for others
}

func (seq *Sequencer) validateAndProcessIncomingBlock(b *common.BlockWithTransactions) error {
	if len(b.Marshal()) > BlockMaximumSize {
		return nil
	}
	if b.Number > seq.nextBlockNumber() {
		seq.incomingBlocks <- b
		return nil
	}
	if b.Number < seq.nextBlockNumber() {
		old, err := seq.node.persistStore.ReadBlockWithTransactions(b.Number)
		if err != nil {
			panic(err)
		}
		if old.PayloadHash() != b.PayloadHash() {
			return seq.voteTheBlockSlash(b, 100)
		}
		return nil
	}

	producer := seq.getNextProducer(b.Timestamp)
	if !b.Verify(producer.Signer.PublicSpendKey) {
		// for now because the node pleding, removing accepting issue
		// we don't have a correct order here
		panic(producer.Signer.PublicSpendKey.String())
		// return nil
	}
	if producer.IdForNetwork != b.NodeId {
		return seq.voteTheBlockSlash(b, 100)
	}
	if b.Number > 0 && b.Previous != seq.CurrentBlock.PayloadHash() {
		return seq.voteTheBlockSlash(b, 100)
	}

	for _, tx := range b.Transactions {
		err := seq.node.cachePutTransaction(tx)
		if err != nil {
			panic(err)
		}
	}
	var pending bool
	var snaps []crypto.Hash
	for _, s := range b.Snapshots {
		if s.NodeId == b.NodeId {
			return seq.voteTheBlockSlash(b, 100)
		}
		snaps = append(snaps, s.PayloadHash())
		old, err := seq.node.persistStore.ReadSnapshot(s.PayloadHash())
		if err != nil {
			panic(err)
		}
		if old != nil {
			continue
		}
		pending = true
		err = seq.node.VerifyAndQueueAppendSnapshotFinalization(b.NodeId, s)
		if err != nil {
			panic(err)
		}
		// TODO vote 13439XIN slash if the snapshot has no valid signature
		// and the time elapsed for more than 1 minute
	}

	// snapshot not processed by cosi
	if pending {
		seq.incomingBlocks <- b
		return nil
	}

	// we check here, because we must wait all snapshots finalized
	// so that the graph timestamp is updated
	if b.Timestamp > seq.node.GraphTimestamp {
		seq.incomingBlocks <- b
		return nil
		// TODO if too long a time then votereturn seq.voteTheBlockSlash(b, 100)
	}

	// TODO check snapshots sequenced already, then reduce the mint reward
	sequenced, err := seq.node.persistStore.CheckSnapshotsSequencedIn(snaps)
	logger.Printf("CheckSnapshotsSequencedIn(%s, %d, %d) => %d %v",
		b.NodeId, b.Number, b.Timestamp, len(sequenced), err)
	if err != nil {
		panic(err)
	}

	err = seq.node.persistStore.WriteBlock(&b.Block, 0)
	logger.Printf("sequencer.validateAndProcessIncomingBlock(%s) => write %v", seq.node.IdForNetwork, b)
	if err != nil {
		panic(err)
	}
	seq.CurrentBlock = &b.Block
	return nil

	// 1. check snapshots unseuqnced, otherwise reduce reward
	// 2. check snapshots not lead by the producer, otherwise slash
	// 3. check block size

	// write txs in cache storage
	// append all snapshots to cosi
	// marshal the total block and snapshots size not BlockAndSnapshotsMaximumSize
	// if the block invalid, just skip or slash
	// if block valid, check snapshots finalized in my own storage
	// if finalized, then write the block
	// if not, then append this block to the incoming blocks queue again

	// iamSynced is useful, because sometimes I'm just syncing the old blocks
	// but I still need to verify their signatures according to the producers
	// list at the block time, so what is the point of this param?
}

func (seq *Sequencer) voteTheBlockSlash(b *common.BlockWithTransactions, amount uint) error {
	if !seq.checkSynced() {
		return nil
	}
	switch amount {
	case 100:
	case 13439:
	default:
		panic(amount)
	}
	// TODO send slash vote
	// maybe I just return nil for this, and the malicious block producer
	// will timeout finally, and it will be slashed due to timeout
	// for just 100XIN
	// 1. snapshot is leaded, just ignore? and timeout for 100XIN
	// 2. invalid block size, just ignore? and timeout for 100XIN
	//
	// we won't wait even for 100XIN slash, just make the normal
	// slash transaction as below.
	//
	// but some slash could be 13439 if it includes invalid snapshots
	// for this, we vote node by node, start a vote transaction and
	// send it to all other nodes, just like a normal transaciton,
	// then if this transaction is finalized, the node will be
	// slashed for 13439XIN.
	//
	// But here we must be sure to check the snapshot signatures,
	// If I can't find the snapshot in my storage, I will check the
	// snapshot is finalized or not, if not finalized, then I will
	// vote. If finalized I will just keep waiting, and never vote
	// otherwise I will be slashed for adversary vote.
	// During the wait, I could send some requests to other nodes
	// or I could just produce an empty block? no empty block.
	panic(seq.node.IdForNetwork)
}

func (node *Node) GetBlockProducers(blockTime uint64) []*CNode {
	var nodes []*CNode
	accepted := node.ListWorkingAcceptedNodes(blockTime)
	for _, cn := range accepted {
		if node.ConsensusReady(cn, blockTime) {
			nodes = append(nodes, cn)
		}
	}
	return nodes
}

func (node *Node) CheckNeighbor(nodeId crypto.Hash) bool {
	neighbors := node.Peer.Neighbors()
	return slices.ContainsFunc(neighbors, func(p *p2p.Peer) bool {
		return p.IdForNetwork == nodeId
	})
}

func (seq *Sequencer) doSlashingVote() {
	if !seq.checkSynced() {
		return
	}
	// TODO
	// check synced
	// if not synced
	// send block requests to 2 nodes that are higher than me
	// the block requests should include block number range
	// e.g. the minimum and maximum number, max 512
	// and they will send 512 blocks all together to me
	// then also with all the snapshots and transactions
	// then just disable the old topology sync strategy
	// if synced
	// check current signer has produced
	// otherwise broadcast a slash vote
}

func (node *Node) QueueBlockHeader(peerId crypto.Hash, data []byte) error {
	if len(data) != 48 {
		panic(len(data))
	}
	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	head := &NodeHeadRequest{
		Number:      binary.BigEndian.Uint64(data[32:40]),
		SyncRequest: binary.BigEndian.Uint64(data[40:]),
	}
	copy(head.NodeId[:], data[:32])
	select {
	case node.sequencer.incomingHeads <- head:
		return nil
	case <-timer.C:
		return fmt.Errorf("QueueBlockHeader(%s, %x) => timeout", peerId, data)
	}
}

func (node *Node) QueueBlocks(peerId crypto.Hash, data []byte) error {
	count := binary.BigEndian.Uint16(data[:2])
	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	data = data[2:]
	for range count {
		l := binary.BigEndian.Uint32(data[:4])
		b, err := common.UnmarshalBlockWithTransactions(data[4 : 4+l])
		if err != nil {
			return err
		}
		select {
		case node.sequencer.incomingBlocks <- b:
		case <-timer.C:
			return fmt.Errorf("QueueBlocks(%s, %d) => timeout", peerId, count)
		}
		data = data[4+l:]
	}
	return nil
}
