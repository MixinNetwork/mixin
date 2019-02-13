package network

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (me *Peer) compareRoundGraphAndGetTopologicalOffset(local, remote []SyncPoint) (uint64, error) {
	localFilter := make(map[crypto.Hash]*SyncPoint)
	for _, p := range local {
		localFilter[p.NodeId] = &p
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
			panic(fmt.Errorf("local final should never has zero snapshots %s:%d %s:%d", l.NodeId.String(), l.Number, r.NodeId.String(), r.Number))
		}
		s := ss[len(ss)-1]
		topo := s.Timestamp // FIXME s.TopologicalOrder
		if topo == 0 {
			topo = 1
		}
		if offset == 0 || topo < offset {
			offset = topo
		}
	}
	return offset, nil
}

func (me *Peer) syncToNeighborSince(p *Peer, offset uint64, filter map[crypto.Hash]time.Time) (uint64, error) {
	snapshots, err := me.handle.ReadSnapshotsSinceTopology(offset, 1000)
	if err != nil {
		return offset, err
	}
	for _, s := range snapshots {
		hash := s.Transaction.PayloadHash()
		if filter[hash].Add(time.Duration(config.SnapshotRoundGap)).After(time.Now()) {
			continue
		}
		err := p.SendData(buildSnapshotMessage(&s.Snapshot))
		if err != nil {
			return offset, err
		}
		offset = s.TopologicalOrder
		filter[hash] = time.Now()
	}
	return offset, nil
}

func (me *Peer) syncToNeighborLoop(p *Peer) {
	var offset uint64
	filter := make(map[crypto.Hash]time.Time)
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
		off, err := me.syncToNeighborSince(p, offset, filter)
		if err != nil {
			logger.Printf("GRAPH SYNC TO %s %s", p.IdForNetwork.String(), err.Error())
		}
		if off > 0 {
			offset = off
		}
	}
}
