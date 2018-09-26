package kernel

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) handlePledgeTransactionConfirmation() error {
	return node.managePeersList()
}

func (node *Node) handleRelcaimTransactionConfirmation() error {
	return node.managePeersList()
}

func (node *Node) managePeersList() error {
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
	peers := make([]*Peer, 0)
	for _, in := range inputs {
		if in.Address == node.Account.String() {
			continue
		}
		acc, err := common.NewAddressFromString(in.Address)
		if err != nil {
			return err
		}
		peers = append(peers, NewPeer(acc, in.Host))
	}
	node.Peers = peers
	for _, p := range node.Peers {
		if p.Account.String() == node.Account.String() {
			continue
		}
		go func() {
			for {
				err := node.openPeerStream(p)
				if err != nil {
					logger.Println("election routine peer error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
	}
	return nil
}
