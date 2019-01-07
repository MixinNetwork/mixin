package kernel

import (
	"encoding/json"
	"io/ioutil"

	"github.com/MixinNetwork/mixin/common"
)

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

func (node *Node) connectNeighbors() error {
	f, err := ioutil.ReadFile(node.configDir + "/nodes.json")
	if err != nil {
		return err
	}
	var inputs []struct {
		Address string `json:"address"`
		Host    string `json:"host"`
	}
	err = json.Unmarshal(f, &inputs)
	if err != nil {
		return err
	}
	for _, in := range inputs {
		if in.Address == node.Account.String() {
			continue
		}
		acc, err := common.NewAddressFromString(in.Address)
		if err != nil {
			return err
		}
		node.Peer.AddNeighbor(node.networkId, acc, in.Host)
	}

	node.Peer.ConnectNeighbors()
	return nil
}
