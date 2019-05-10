package kernel

import "github.com/MixinNetwork/mixin/common"

func (node *Node) manageConsensusNodesList(tx *common.VersionedTransaction) error {
	switch tx.TransactionType() {
	case common.TransactionTypeNodePledge, common.TransactionTypeNodeAccept, common.TransactionTypeNodeDepart, common.TransactionTypeNodeRemove:
		err := node.LoadConsensusNodes()
		if err != nil {
			return err
		}
		graph, err := LoadRoundGraph(node.store, node.networkId, node.IdForNetwork)
		if err != nil {
			return err
		}
		node.Graph = graph
	}
	return nil
}
