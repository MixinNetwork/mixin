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
	Account     common.Address
	Peers       []*Peer
	RoundHash   crypto.Hash
	RoundNumber uint64
	Timestamp   uint64
	Address     string
	Graph       *RoundGraph
	TopoCounter *TopologicalSequence

	syncrhoinized bool
	networkId     crypto.Hash
	store         storage.Store
	transport     network.Transport
	mempoolChan   chan *common.Snapshot
	filter        map[crypto.Hash]bool
	configDir     string
}

func setupNode(store storage.Store, addr string, dir string) (*Node, error) {
	var node = &Node{
		Address:     addr,
		store:       store,
		mempoolChan: make(chan *common.Snapshot, MempoolSize),
		filter:      make(map[crypto.Hash]bool),
		configDir:   dir,
		TopoCounter: getTopologyCounter(store),
	}

	networkId, err := node.loadGenesis(dir)
	if err != nil {
		return nil, err
	}
	node.networkId, err = crypto.HashFromString(networkId)
	if err != nil {
		return nil, err
	}

	err = node.loadNodeStateFromStore()
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

	err = node.managePeersList()
	if err != nil {
		return nil, err
	}

	logger.Printf("Listen:\t%s\n", node.Address)
	logger.Printf("Account:\t%s\n", node.Account.String())
	logger.Printf("View Key:\t%s\n", node.Account.PrivateViewKey.String())
	logger.Printf("Spend Key:\t%s\n", node.Account.PrivateSpendKey.String())
	logger.Printf("Network:\t%s\n", node.networkId.String())
	logger.Printf("Node Id:\t%s\n", node.IdForNetwork().String())
	return node, nil
}

func (node *Node) IdForNetwork() crypto.Hash {
	nodeId := node.Account.Hash()
	networkId := node.networkId
	return crypto.NewHash(append(networkId[:], nodeId[:]...))
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

func (node *Node) feedMempool(s *common.Snapshot) error {
	hash := s.Transaction.Hash()
	if node.filter[hash] {
		return nil
	}
	node.filter[hash] = true
	node.mempoolChan <- s
	return nil
}

func (node *Node) ConsumeMempool() error {
	return nil
}
