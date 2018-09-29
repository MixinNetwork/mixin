package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) compareRoundGraphAndGetTopologicalOffset(local, remote []FinalRound) (uint64, error) {
	localFilter := make(map[crypto.Hash]*FinalRound)
	for _, r := range local {
		localFilter[r.NodeId] = &r
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

		ss, err := node.store.SnapshotsListForNodeRound(r.NodeId, r.Number)
		if err != nil {
			return offset, err
		}
		s, err := node.store.SnapshotsReadByTransactionHash(ss[0].Transaction.Hash())
		if err != nil {
			return offset, err
		}
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

func (node *Node) SyncFinalGraphToAllPeers() {
	for _, p := range node.GossipPeers {
		go node.syncToPeerLoop(p)
	}
}

func (node *Node) syncToPeerSince(p *Peer, offset uint64, filter map[crypto.Hash]bool) (uint64, error) {
	snapshots, err := node.store.SnapshotsListTopologySince(offset, 1000)
	if err != nil {
		return offset, err
	}
	for _, s := range snapshots {
		hash := s.Transaction.Hash()
		if filter[hash] {
			continue
		}
		err := p.Send(buildSnapshotMessage(&s.Snapshot))
		if err != nil {
			return offset, err
		}
		offset = s.TopologicalOrder
		filter[hash] = true
	}
	return offset, nil
}

func (node *Node) syncToPeerLoop(p *Peer) {
	var offset uint64
	filter := make(map[crypto.Hash]bool)
	for {
		select {
		case g := <-p.GraphChan:
			off, err := node.compareRoundGraphAndGetTopologicalOffset(node.Graph.FinalCache, g)
			if err != nil {
				logger.Println("GRAPH COMPARE WITH %s %s", p.IdForNetwork.String(), err.Error())
			}
			if off > 0 {
				offset = off
			}
		case <-time.After(100 * time.Millisecond):
		}
		if offset == 0 {
			continue
		}
		off, err := node.syncToPeerSince(p, offset, filter)
		if err != nil {
			logger.Println("GRAPH SYNC TO %s %s", p.IdForNetwork.String(), err.Error())
		}
		if off > 0 {
			offset = off
		}
	}
}
