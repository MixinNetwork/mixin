package kernel

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io/ioutil"
	"time"

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
	Graph          *RoundGraph
	TopoCounter    *TopologicalSequence
	SnapshotsPool  map[crypto.Hash]*common.Snapshot
	ConsensusPool  map[crypto.Hash]time.Time
	Peer           *network.Peer

	networkId   crypto.Hash
	store       storage.Store
	mempoolChan chan *common.Snapshot
	configDir   string
}

func SetupNode(store storage.Store, addr string, dir string) (*Node, error) {
	var node = &Node{
		ConsensusNodes: make([]common.Address, 0),
		SnapshotsPool:  make(map[crypto.Hash]*common.Snapshot),
		ConsensusPool:  make(map[crypto.Hash]time.Time),
		store:          store,
		mempoolChan:    make(chan *common.Snapshot, MempoolSize),
		configDir:      dir,
		TopoCounter:    getTopologyCounter(store),
	}

	err := node.LoadNodeState()
	if err != nil {
		return nil, err
	}

	err = node.LoadGenesis(dir)
	if err != nil {
		return nil, err
	}

	err = node.LoadConsensusNodes()
	if err != nil {
		return nil, err
	}

	graph, err := LoadRoundGraph(node.store)
	if err != nil {
		return nil, err
	}
	node.Graph = graph

	node.Peer = network.NewPeer(node, node.IdForNetwork, addr)
	err = node.AddNeighborsFromConfig()
	if err != nil {
		return nil, err
	}

	logger.Printf("Listen:\t%s\n", addr)
	logger.Printf("Account:\t%s\n", node.Account.String())
	logger.Printf("View Key:\t%s\n", node.Account.PrivateViewKey.String())
	logger.Printf("Spend Key:\t%s\n", node.Account.PrivateSpendKey.String())
	logger.Printf("Network:\t%s\n", node.networkId.String())
	logger.Printf("Node Id:\t%s\n", node.IdForNetwork.String())
	logger.Printf("Topology:\t%d\n", node.TopoCounter.seq)
	return node, nil
}

func (node *Node) LoadNodeState() error {
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

func (node *Node) LoadConsensusNodes() error {
	nodes, err := node.store.SnapshotsReadAcceptedNodes()
	if err != nil {
		return err
	}
	node.ConsensusNodes = nodes
	return nil
}

func (node *Node) AddNeighborsFromConfig() error {
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
		node.Peer.AddNeighbor(acc.Hash().ForNetwork(node.networkId), in.Host)
	}

	return nil
}

func (node *Node) ListenNeighbors() error {
	return node.Peer.ListenNeighbors()
}

func (node *Node) NetworkId() crypto.Hash {
	return node.networkId
}

func (node *Node) BuildGraph() []network.SyncPoint {
	points := make([]network.SyncPoint, 0)
	for _, c := range node.Graph.FinalCache {
		points = append(points, network.SyncPoint{
			NodeId: c.NodeId,
			Number: c.Number,
			Start:  c.Start,
		})
	}
	return points
}

func (node *Node) BuildAuthenticationMessage() []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(time.Now().Unix()))
	hash := node.Account.Hash()
	data = append(data, hash[:]...)
	sig := node.Account.PrivateSpendKey.Sign(data)
	return append(data, sig[:]...)
}

func (node *Node) Authenticate(msg []byte) (crypto.Hash, error) {
	ts := binary.BigEndian.Uint64(msg[:8])
	if time.Now().Unix()-int64(ts) > 3 {
		return crypto.Hash{}, errors.New("peer authentication message timeout")
	}

	for _, cn := range node.ConsensusNodes {
		peerId := cn.Hash()
		if !bytes.Equal(peerId[:], msg[8:40]) {
			continue
		}
		var sig crypto.Signature
		copy(sig[:], msg[40:])
		if cn.PublicSpendKey.Verify(msg[:40], sig) {
			return peerId.ForNetwork(node.networkId), nil
		}
		break
	}

	return crypto.Hash{}, errors.New("peer authentication message signature invalid")
}

func (node *Node) FeedMempool(peer *network.Peer, s *common.Snapshot) error {
	if peer.IdForNetwork == node.IdForNetwork {
		node.mempoolChan <- s
		return nil
	}

	for _, cn := range node.ConsensusNodes {
		idForNetwork := cn.Hash().ForNetwork(node.networkId)
		if idForNetwork != peer.IdForNetwork {
			continue
		}
		if s.CheckSignature(cn.PublicSpendKey) {
			node.mempoolChan <- s
		}
		break
	}
	return nil
}

func (node *Node) ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	return node.store.SnapshotsReadSnapshotsSinceTopology(offset, count)
}

func (node *Node) ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error) {
	return node.store.SnapshotsReadSnapshotsForNodeRound(nodeIdWithNetwork, round)
}

func (node *Node) ReadSnapshotByTransactionHash(hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error) {
	return node.store.SnapshotsReadSnapshotByTransactionHash(hash)
}

func (node *Node) ConsumeMempool() error {
	for {
		select {
		case s := <-node.mempoolChan:
			err := node.handleSnapshotInput(s)
			if err != nil {
				return err
			}
		}
	}
}
