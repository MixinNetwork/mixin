package kernel

import (
	"crypto/rand"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/network"
	"github.com/MixinNetwork/mixin/storage"
)

const (
	MempoolSize = 8192
)

type Node struct {
	IdForNetwork   crypto.Hash
	Account        common.Address
	ConsensusNodes []common.Address
	GossipPeers    map[crypto.Hash]*Peer
	Address        string
	Graph          *RoundGraph
	TopoCounter    *TopologicalSequence
	SnapshotsPool  map[crypto.Hash]*common.Snapshot

	networkId   crypto.Hash
	store       storage.Store
	transport   network.Transport
	mempoolChan chan *common.Snapshot
	configDir   string
}

func setupNode(store storage.Store, addr string, dir string) (*Node, error) {
	var node = &Node{
		Address:        addr,
		ConsensusNodes: make([]common.Address, 0),
		GossipPeers:    make(map[crypto.Hash]*Peer),
		SnapshotsPool:  make(map[crypto.Hash]*common.Snapshot),
		store:          store,
		mempoolChan:    make(chan *common.Snapshot, MempoolSize),
		configDir:      dir,
		TopoCounter:    getTopologyCounter(store),
	}

	err := node.loadNodeStateFromStore()
	if err != nil {
		return nil, err
	}

	err = node.loadGenesis(dir)
	if err != nil {
		return nil, err
	}

	graph, err := loadRoundGraph(node.store)
	if err != nil {
		return nil, err
	}
	node.Graph = graph

	transport, err := network.NewQuicServer(addr, node.Account.PrivateSpendKey)
	if err != nil {
		return nil, err
	}
	node.transport = transport

	err = node.loadPeersList()
	if err != nil {
		return nil, err
	}

	logger.Printf("Listen:\t%s\n", node.Address)
	logger.Printf("Account:\t%s\n", node.Account.String())
	logger.Printf("View Key:\t%s\n", node.Account.PrivateViewKey.String())
	logger.Printf("Spend Key:\t%s\n", node.Account.PrivateSpendKey.String())
	logger.Printf("Network:\t%s\n", node.networkId.String())
	logger.Printf("Node Id:\t%s\n", node.IdForNetwork.String())
	logger.Printf("Topology:\t%d\n", node.TopoCounter.seq)
	return node, nil
}

func (node *Node) loadNodeStateFromStore() error {
	const stateKeyAccount = "account"
	var acc common.Address
	found, err := node.store.StateGet(stateKeyAccount, &acc)
	if err != nil {
		return err
	} else if !found {
		b := make([]byte, 32)
		_, err := rand.Read(b)
		if err != nil {
			panic(err)
		}
		acc = common.NewAddressFromSeed(b)
	}
	err = node.store.StateSet(stateKeyAccount, acc)
	if err != nil {
		return err
	}
	node.Account = acc
	return nil
}
