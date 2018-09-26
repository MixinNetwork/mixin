package kernel

import "github.com/MixinNetwork/mixin/crypto"

type RoundCache struct {
}

func (node *Node) loadRound() error {
	snapshots, err := node.store.SnapshotsListForNodeRound(node.IdForNetwork(), node.RoundNumber)
	if err != nil {
		return err
	}

	var hashes []byte
	for _, s := range snapshots {
		h := crypto.NewHash(s.Payload())
		hashes = append(hashes, h[:]...)
	}
	node.RoundHash = crypto.NewHash(hashes)
	return nil
}

func (node *Node) loadRoundForPeer(peer *Peer) (crypto.Hash, error) {
	peerId := peer.Account.Hash()
	networkId := node.networkId
	peerIdForNetwork := crypto.NewHash(append(networkId[:], peerId[:]...))

	snapshots, err := node.store.SnapshotsListForNodeRound(peerIdForNetwork, peer.RoundNumber)
	if err != nil {
		return crypto.Hash{}, err
	}

	var hashes []byte
	for _, s := range snapshots {
		h := crypto.NewHash(s.Payload())
		hashes = append(hashes, h[:]...)
	}
	return crypto.NewHash(hashes), nil
}

func (node *Node) commitRound() error {
	return nil
}
