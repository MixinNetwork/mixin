package network

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (me *Peer) cacheReadSnapshotsForNodeRound(nodeId crypto.Hash, number uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	return me.handle.ReadSnapshotsForNodeRound(nodeId, number)
}

func (me *Peer) cacheReadSnapshotsSinceTopology(offset, limit uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	return me.handle.ReadSnapshotsSinceTopology(offset, limit)
}

func (me *Peer) compareRoundGraphAndGetTopologicalOffset(p *Peer, local, remote []*SyncPoint) (uint64, error) {
	remoteFilter := make(map[crypto.Hash]*SyncPoint)
	for _, p := range remote {
		remoteFilter[p.NodeId] = p
	}

	var offset uint64

	for _, l := range local {
		r := remoteFilter[l.NodeId]
		if r == nil || r.Number > l.Number {
			continue
		}
		number := r.Number + 2 // because the node may be stale or removed, and with cache
		logger.Verbosef("network.sync compareRoundGraphAndGetTopologicalOffset %s try %s:%d\n", p.IdForNetwork, l.NodeId, number)

		ss, err := me.cacheReadSnapshotsForNodeRound(l.NodeId, number)
		if err != nil {
			return offset, err
		}
		if len(ss) == 0 {
			logger.Verbosef("network.sync compareRoundGraphAndGetTopologicalOffset %s local round empty %s:%d:%d\n", p.IdForNetwork, l.NodeId, number, l.Number)
			continue
		}
		topo := ss[0].TopologicalOrder
		if offset == 0 || topo < offset {
			offset = topo
		}
	}
	return offset, nil
}

func (me *Peer) syncToNeighborSince(graph map[crypto.Hash]*SyncPoint, p *Peer, offset uint64) (uint64, error) {
	logger.Verbosef("network.sync syncToNeighborSince %s %d\n", p.IdForNetwork, offset)
	limit := 200
	snapshots, err := me.cacheReadSnapshotsSinceTopology(offset, uint64(limit))
	if err != nil {
		return offset, err
	}
	for _, s := range snapshots {
		var remoteRound uint64
		if r := graph[s.NodeId]; r != nil {
			remoteRound = r.Number
		}
		if s.RoundNumber < remoteRound {
			offset = s.TopologicalOrder
			continue
		}
		if s.RoundNumber >= remoteRound+config.SnapshotReferenceThreshold*2 {
			return offset, fmt.Errorf("FUTURE %s %d %d", s.NodeId, s.RoundNumber, remoteRound)
		}
		err := me.SendSnapshotFinalizationMessage(p.IdForNetwork, &s.Snapshot)
		if err != nil {
			return offset, err
		}
		offset = s.TopologicalOrder
	}
	time.Sleep(100 * time.Millisecond)
	if len(snapshots) < limit {
		return offset, fmt.Errorf("EOF")
	}
	return offset, nil
}

func (me *Peer) syncHeadRoundToRemote(local, remote map[crypto.Hash]*SyncPoint, p *Peer, nodeId crypto.Hash) {
	var localFinal, remoteFinal uint64
	if r := remote[nodeId]; r != nil {
		remoteFinal = r.Number
	}
	if l := local[nodeId]; l != nil {
		localFinal = l.Number
	}
	if remoteFinal > localFinal {
		return
	}
	logger.Verbosef("network.sync syncHeadRoundToRemote %s %s:%d\n", p.IdForNetwork, nodeId, remoteFinal)
	for i := remoteFinal; i <= remoteFinal+config.SnapshotReferenceThreshold+2; i++ {
		ss, _ := me.cacheReadSnapshotsForNodeRound(nodeId, i)
		for _, s := range ss {
			me.SendSnapshotFinalizationMessage(p.IdForNetwork, &s.Snapshot)
		}
	}
}

func (me *Peer) syncToNeighborLoop(p *Peer) {
	defer close(p.stn)

	for !me.closing && !p.closing {
		graph, offset := me.getSyncPointOffset(p)
		logger.Verbosef("network.sync syncToNeighborLoop getSyncPointOffset %s %d %v\n", p.IdForNetwork, offset, graph != nil)

		if me.gossipRound.Get(p.IdForNetwork) == nil {
			continue
		}

		for !me.closing && !p.closing && offset > 0 {
			off, err := me.syncToNeighborSince(graph, p, offset)
			if err != nil {
				logger.Verbosef("network.sync syncToNeighborLoop syncToNeighborSince %s %d DONE with %s", p.IdForNetwork, offset, err)
				break
			}
			offset = off
		}

		if graph != nil {
			points := me.handle.BuildGraph()
			nodes := me.handle.ReadAllNodesWithoutState()
			local := make(map[crypto.Hash]*SyncPoint)
			for _, n := range points {
				local[n.NodeId] = n
			}
			for _, n := range nodes {
				me.syncHeadRoundToRemote(local, graph, p, n)
			}
		}
	}
}

func (me *Peer) getSyncPointOffset(p *Peer) (map[crypto.Hash]*SyncPoint, uint64) {
	var offset uint64
	var graph map[crypto.Hash]*SyncPoint

	startAt := time.Now()
	for !me.closing && !p.closing {
		item, err := p.syncRing.Poll(false)
		if err != nil {
			break
		} else if item == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		g := item.([]*SyncPoint)
		graph = make(map[crypto.Hash]*SyncPoint)
		for _, r := range g {
			graph[r.NodeId] = r
		}
		off, err := me.compareRoundGraphAndGetTopologicalOffset(p, me.handle.BuildGraph(), g)
		if err != nil {
			logger.Printf("network.sync compareRoundGraphAndGetTopologicalOffset %s error %s\n", p.IdForNetwork, err.Error())
		}
		if off > 0 {
			offset = off
		}
		if startAt.Add(time.Second).Before(time.Now()) {
			return graph, offset
		}
	}

	return nil, 0
}
