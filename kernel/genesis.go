package kernel

import "github.com/MixinNetwork/mixin/common"

func (node *Node) LoadGenesis(gns *common.Genesis) error {
	node.Epoch = gns.EpochTimestamp()
	node.networkId = gns.NetworkId()
	node.IdForNetwork = node.Signer.Hash().ForNetwork(node.networkId)
	for _, in := range gns.Nodes {
		id := in.Signer.Hash().ForNetwork(node.networkId)
		node.genesisNodesMap[id] = true
		node.genesisNodes = append(node.genesisNodes, id)
	}

	rounds, snapshots, transactions, err := gns.BuildSnapshots()
	if err != nil {
		return err
	}

	loaded, err := node.persistStore.CheckGenesisLoad(snapshots)
	if err != nil || loaded {
		return err
	}

	return node.persistStore.LoadGenesis(rounds, snapshots, transactions)
}
