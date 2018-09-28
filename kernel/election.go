package kernel

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
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

func (node *Node) loadPeersList() error {
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
		peer := NewPeer(acc, in.Host)
		peerId := peer.Account.Hash()
		peer.IdForNetwork = crypto.NewHash(append(node.networkId[:], peerId[:]...))
		node.GossipPeers[peer.IdForNetwork] = peer
	}
	for _, p := range node.GossipPeers {
		if p.Address == node.Address {
			continue
		}
		go func(peer *Peer) {
			for {
				err := node.openPeerStream(peer)
				if err != nil {
					logger.Println("election routine peer error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}(p)
	}
	return nil
}
