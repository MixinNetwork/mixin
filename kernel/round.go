package kernel

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/storage"
)

type Round struct {
	NodeId crypto.Hash
	Number uint64
	Start  uint64
	Hash   crypto.Hash
}

func loadRoundForNode(store storage.Store, nodeIdWithNetwork crypto.Hash) (*Round, error) {
	meta, err := store.SnapshotsRoundMetaForNode(nodeIdWithNetwork)
	if err != nil {
		return nil, err
	}

	round := &Round{
		NodeId: nodeIdWithNetwork,
		Number: meta[0],
		Start:  meta[1],
	}

	snapshots, err := store.SnapshotsListForNodeRound(round.NodeId, round.Number)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, round.Number)
	hashes := append(round.NodeId[:], buf...)
	for _, s := range snapshots {
		h := crypto.NewHash(s.Payload())
		hashes = append(hashes, h[:]...)
	}
	round.Hash = crypto.NewHash(hashes)
	return round, nil
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
