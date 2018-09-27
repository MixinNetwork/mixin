package kernel

import (
	"time"

	"github.com/MixinNetwork/mixin/storage"
)

func Loop(store storage.Store, addr string, dir string) error {
	node, err := setupNode(store, addr, dir)
	if err != nil {
		return err
	}
	panicGo(node.ListenPeers)
	node.syncSnapshots()
	panicGo(node.ConsumeMempool)
	return node.ConsumeQueue()
}

func (node *Node) syncSnapshots() {
	for _, p := range node.ConsensusPeers {
		node.readGraphHeadFromPeer(p)
	}
	time.Sleep(1 * time.Second)
	node.syncrhoinized = true
}

// for each peer, read the nodes list
// and the node head info, e.g. round number
// node may not be an active peer
// node is just a graph branch in the structure
func (node *Node) readGraphHeadFromPeer(p *Peer) {
}

// after read head info, I will request the latest snapshot list from some peers
// the peers then will send the snapshots to me
// I will do the snapshot validation just like normal validation
// after the snapshots synced to the latest graph head within 3 seconds
// should update me to synchronized?
