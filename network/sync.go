package network

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (me *Peer) cacheReadSnapshotsForNodeRound(nodeId crypto.Hash, final uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	key := []byte(fmt.Sprintf("SFNR%s:%d", nodeId.String(), final))
	data := me.storeCache.Get(nil, key)
	if len(data) == 0 {
		ss, err := me.handle.ReadSnapshotsForNodeRound(nodeId, final)
		if err != nil {
			return nil, err
		}
		me.storeCache.Set(key, common.MsgpackMarshalPanic(ss))
		return ss, nil
	}
	var ss []*common.SnapshotWithTopologicalOrder
	err := common.MsgpackUnmarshal(data, &ss)
	return ss, err
}

func (me *Peer) cacheReadSnapshotsSinceTopology(offset, limit uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	key := []byte(fmt.Sprintf("SSTME%d-%d", offset, limit))
	data := me.storeCache.Get(nil, key)
	if len(data) == 0 {
		ss, err := me.handle.ReadSnapshotsSinceTopology(offset, limit)
		if err != nil {
			return nil, err
		}
		if uint64(len(ss)) == limit {
			me.storeCache.Set(key, common.MsgpackMarshalPanic(ss))
		}
		return ss, nil
	}
	var ss []*common.SnapshotWithTopologicalOrder
	err := common.MsgpackUnmarshal(data, &ss)
	return ss, err
}

func (me *Peer) compareRoundGraphAndGetTopologicalOffset(local, remote []*SyncPoint) (uint64, error) {
	remoteFilter := make(map[crypto.Hash]*SyncPoint)
	for _, p := range remote {
		remoteFilter[p.NodeId] = p
	}

	var future bool
	var offset uint64

	for _, l := range local {
		r := remoteFilter[l.NodeId]
		if r == nil {
			future = true
			break
		}
		if l.Number >= r.Number+config.SnapshotReferenceThreshold/2 {
			future = true
			break
		}
	}
	if !future {
		return offset, nil
	}

	for _, l := range local {
		r := remoteFilter[l.NodeId]
		if r != nil && r.Number > l.Number {
			continue
		}
		number := uint64(0)
		if r != nil {
			number = r.Number
		}

		ss, err := me.cacheReadSnapshotsForNodeRound(l.NodeId, number)
		if err != nil {
			return offset, err
		}
		if len(ss) == 0 {
			panic(fmt.Errorf("final should never have zero snapshots %s:%d:%d", l.NodeId.String(), number, l.Number))
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
		if r := graph[s.NodeId]; r != nil && s.RoundNumber <= r.Number {
			offset = s.TopologicalOrder
			continue
		}
		var remoteRound uint64
		if r := graph[s.NodeId]; r != nil {
			remoteRound = r.Number
		}
		if s.RoundNumber >= remoteRound+config.SnapshotSyncRoundThreshold*2 {
			return offset, fmt.Errorf("FUTURE %d %d", s.RoundNumber, remoteRound)
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
