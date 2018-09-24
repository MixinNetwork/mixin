package kernel

import "github.com/MixinNetwork/mixin/crypto"

func (node *Node) RoundHash() (crypto.Hash, error) {
	snapshots, err := node.store.SnapshotsForNodeRound(node.IdForNetwork(), node.RoundNumber)
	if err != nil {
		return crypto.Hash{}, err
	}

	var hashes []byte
	for _, s := range snapshots {
		h := s.Hash()
		hashes = append(hashes, h[:]...)
	}
	return crypto.NewHash(hashes), nil
}

func (node *Node) RoundHashForPeer(peer *Peer) (crypto.Hash, error) {
	peerId := peer.Id.Hash()
	networkId := node.networkId
	peerIdForNetwork := crypto.NewHash(append(networkId[:], peerId[:]...))

	snapshots, err := node.store.SnapshotsForNodeRound(peerIdForNetwork, peer.RoundNumber)
	if err != nil {
		return crypto.Hash{}, err
	}

	var hashes []byte
	for _, s := range snapshots {
		h := s.Hash()
		hashes = append(hashes, h[:]...)
	}
	return crypto.NewHash(hashes), nil
}
