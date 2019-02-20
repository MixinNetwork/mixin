package network

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (me *Peer) compareRoundGraphAndGetTopologicalOffset(local, remote []*SyncPoint) (uint64, error) {
	localFilter := make(map[crypto.Hash]*SyncPoint)
	for _, p := range local {
		localFilter[p.NodeId] = p
	}

	var offset uint64
	for _, r := range remote {
		l := localFilter[r.NodeId]
		if l == nil {
			continue
		}
		if l.Number < r.Number {
			continue
		}

		ss, err := me.handle.ReadSnapshotsForNodeRound(r.NodeId, r.Number)
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

func (me *Peer) syncToNeighborSince(p *Peer, offset uint64) (uint64, error) {
	snapshots, err := me.handle.ReadSnapshotsSinceTopology(offset, 1000)
	if err != nil {
		return offset, err
	}
	for _, s := range snapshots {
		err := me.SendSnapshotMessage(p.IdForNetwork, &s.Snapshot, 1)
		if err != nil {
			return offset, err
		}
		offset = s.TopologicalOrder
	}
	if len(snapshots) < 1000 {
		time.Sleep(time.Duration(config.SnapshotRoundGap))
	}
	return offset, nil
}

func (me *Peer) syncToNeighborLoop(p *Peer) {
	var offset uint64
	for {
		select {
		case g := <-p.sync:
			off, err := me.compareRoundGraphAndGetTopologicalOffset(me.handle.BuildGraph(), g)
			if err != nil {
				logger.Printf("GRAPH COMPARE WITH %s %s", p.IdForNetwork.String(), err.Error())
			}
			if off > 0 {
				offset = off
			}
		case <-time.After(100 * time.Millisecond):
		}
		if offset == 0 {
			continue
		}
		off, err := me.syncToNeighborSince(p, offset)
		if err != nil {
			logger.Printf("GRAPH SYNC TO %s %s", p.IdForNetwork.String(), err.Error())
		}
		if off > 0 {
			offset = off
		}
	}
}
