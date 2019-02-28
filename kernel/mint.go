package kernel

import (
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
)

func (node *Node) MintLoop() error {
	for {
		time.Sleep(1 * time.Hour)
		nodes := node.sortMintNodes()

		if !node.checkMintPossibility(nodes) {
			continue
		}

		err := node.tryToMint()
		if err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) tryToMint() error {
	return nil
}

func (node *Node) checkMintPossibility(nodes []*common.Node) bool {
	hours := int(node.Graph.GraphTimestamp / 1000000000 / 3600)
	batch := hours / 24
	if batch < 1 {
		return false
	}
	if hours%24 < 6 || hours%24 > 18 {
		return false
	}

	return true
}

func (node *Node) sortMintNodes() []*common.Node {
	var nodes []*common.Node
	for _, n := range node.ConsensusNodes {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		a := nodes[i].Signer.Hash().ForNetwork(node.networkId)
		b := nodes[j].Signer.Hash().ForNetwork(node.networkId)
		return a.String() < b.String()
	})
	return nodes
}

func checkMintNodesIndex(nodes []*common.Node, m *common.Node) int {
	for i, n := range nodes {
		if n.Signer.String() == m.Signer.String() {
			return i
		}
	}
	return -1
}
