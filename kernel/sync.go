package kernel

func compareRoundGraphAndGetTopologicalOffset(local, remote *RoundGraph) error {
	return nil
}

func (node *Node) pushSnapshotsToplogyToPeer(p *Peer, offset, count uint64) error {
	node.store.SnapshotsListTopologySince(offset, count)
	return nil
}
