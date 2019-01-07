package kernel

func (node *Node) handlePledgeTransactionConfirmation() error {
	return node.manageConsensusNodesList()
}

func (node *Node) handleRelcaimTransactionConfirmation() error {
	return node.manageConsensusNodesList()
}

// consensus nodes may be updated, not same as peers
func (node *Node) manageConsensusNodesList() error {
	return nil
}
