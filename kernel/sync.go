package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/crypto"
)

func compareRoundGraphAndGetTopologicalOffset(local, remote *RoundGraph) error {
	return nil
}

func (node *Node) dummySyncToAllPeers() error {
	var offset uint64
	filter := make(map[crypto.Hash]bool)
	for {
		snapshots, err := node.store.SnapshotsListTopologySince(offset, 100)
		if err != nil {
			return err
		}
		for _, s := range snapshots {
			if filter[s.Transaction.Hash()] {
				continue
			}
			for _, p := range node.GossipPeers {
				err := p.Send(buildSnapshotMessage(&s.Snapshot))
				if err != nil {
					return err
				}
			}
			offset = s.TopologicalOrder
			filter[s.Transaction.Hash()] = true
		}
		if len(snapshots) < 100 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}
