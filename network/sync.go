package network

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/patrickmn/go-cache"
)

func (me *Peer) cacheReadSnapshotsForNodeRound(nodeId crypto.Hash, final uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	key := fmt.Sprintf("SFNR%s:%d", nodeId.String(), final)
	data, found := me.storeCache.Get(key)
	if found {
		return data.([]*common.SnapshotWithTopologicalOrder), nil
	}
	ss, err := me.handle.ReadSnapshotsForNodeRound(nodeId, final)
	if err != nil {
		return nil, err
	}
	me.storeCache.Set(key, ss, cache.DefaultExpiration)
	return ss, nil
}

func (me *Peer) cacheReadSnapshotsSinceTopology(offset, limit uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	key := fmt.Sprintf("SSTME%d-%d", offset, limit)
	data, found := me.storeCache.Get(key)
	if found {
		return data.([]*common.SnapshotWithTopologicalOrder), nil
	}
	ss, err := me.handle.ReadSnapshotsSinceTopology(offset, limit)
	if err != nil {
		return nil, err
	}
	if uint64(len(ss)) == limit {
		me.storeCache.Set(key, ss, cache.DefaultExpiration)
	}
	return ss, nil
}

func (me *Peer) compareRoundGraphAndGetTopologicalOffset(local, remote []*SyncPoint) (uint64, error) {
	localFilter := make(map[crypto.Hash]*SyncPoint)
	for _, p := range local {
		localFilter[p.NodeId] = p
	}

	var future bool
	var offset uint64

	for _, r := range remote {
		l := localFilter[r.NodeId]
		if l == nil {
			continue
		}
		if l.Number >= r.Number {
			future = true
		}
	}
	if !future {
		return offset, nil
	}

	for _, r := range remote {
		l := localFilter[r.NodeId]
		if l == nil {
			continue
		}
		if l.Number < r.Number {
			continue
		}

		ss, err := me.cacheReadSnapshotsForNodeRound(r.NodeId, r.Number)
		if err != nil {
			return offset, err
		}
		if len(ss) == 0 {
			panic(fmt.Errorf("final should never have zero snapshots %s:%d %s:%d", l.NodeId.String(), l.Number, r.NodeId.String(), r.Number))
		}
		s := ss[len(ss)-1]
		topo := s.TopologicalOrder
		if topo == 0 {
			topo = 1
		}
		if offset == 0 || topo < offset {
			offset = topo
		}
	}
	return offset, nil
}

func (me *Peer) syncToNeighborSince(graph map[crypto.Hash]*SyncPoint, p *Peer, offset uint64) (uint64, error) {
	limit := 200
	snapshots, err := me.cacheReadSnapshotsSinceTopology(offset, uint64(limit))
	if err != nil {
		return offset, err
	}
	for _, s := range snapshots {
		if s.RoundNumber <= graph[s.NodeId].Number {
			offset = s.TopologicalOrder
			continue
		}
		err := me.SendSnapshotMessage(p.IdForNetwork, &s.Snapshot, 1)
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

func (me *Peer) syncToNeighborLoop(p *Peer) {
	var offset uint64
	var graph map[crypto.Hash]*SyncPoint
	for !p.closing {
	L:
		for {
			select {
			case g := <-p.sync:
				graph = make(map[crypto.Hash]*SyncPoint)
				for _, r := range g {
					graph[r.NodeId] = r
				}
				off, err := me.compareRoundGraphAndGetTopologicalOffset(me.handle.BuildGraph(), g)
				if err != nil {
					logger.Printf("GRAPH COMPARE WITH %s %s", p.IdForNetwork.String(), err.Error())
				}
				if off > 0 {
					offset = off
				}
			case <-time.After(time.Duration(config.SnapshotRoundGap) / 2):
				break L
			}
		}
		if offset == 0 {
			continue
		}

		time.Sleep(time.Duration(config.SnapshotRoundGap))
		for !p.closing {
			off, err := me.syncToNeighborSince(graph, p, offset)
			if off > 0 {
				offset = off
			}
			if err != nil {
				break
			}
		}
	}
}
