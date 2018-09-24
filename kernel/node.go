package kernel

import (
	"crypto/rand"
	"encoding/json"
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
	Id          common.Address
	Peers       []*Peer
	RoundNumber uint64
	Timestamp   uint64
	Address     string

	networkId   crypto.Hash
	store       storage.Store
	transport   network.Transport
	mempoolChan chan *common.Snapshot
	filter      map[crypto.Hash]bool
	configDir   string
}

func setupNode(store storage.Store, addr string, dir string) (*Node, error) {
	var node = &Node{
		Address:     addr,
		store:       store,
		mempoolChan: make(chan *common.Snapshot, MempoolSize),
		filter:      make(map[crypto.Hash]bool),
		configDir:   dir,
	}

	networkId, err := loadGenesis(store, dir)
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

	transport, err := network.NewQuicServer(addr, node.Id.PrivateSpendKey)
	if err != nil {
		return nil, err
	}
	node.transport = transport

	err = node.rearrangePeersList()
	if err != nil {
		return nil, err
	}

	logger.Printf("Listen:\t%s\n", node.Address)
	logger.Printf("Account:\t%s\n", node.Id.String())
	logger.Printf("View Key:\t%s\n", node.Id.PrivateViewKey.String())
	logger.Printf("Spend Key:\t%s\n", node.Id.PrivateSpendKey.String())
	logger.Printf("Network:\t%s\n", node.networkId.String())
	logger.Printf("Node Id:\t%s\n", node.IdForNetwork().String())
	return node, nil
}

func (node *Node) IdForNetwork() crypto.Hash {
	nodeId := node.Id.Hash()
	networkId := node.networkId
	return crypto.NewHash(append(networkId[:], nodeId[:]...))
}

func (node *Node) loadNodeStateFromStore() error {
	const stateKeyAccount = "account"
	var acc common.Address
	err := node.store.StateGet(stateKeyAccount, &acc)
	if err == storage.ErrorNotFound {
		b := make([]byte, 32)
		_, err := rand.Read(b)
		if err != nil {
			panic(err)
		}
		acc = common.NewAddressFromSeed(b)
	} else if err != nil {
		return err
	}
	err = node.store.StateSet(stateKeyAccount, acc)
	if err != nil {
		return err
	}
	node.Id = acc
	return nil
}

func (node *Node) handleNodeTransactionConfirmation() error {
	return node.rearrangePeersList()
}

func (node *Node) rearrangePeersList() error {
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
		if in.Address == node.Id.String() {
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
		if p.Id.String() == node.Id.String() {
			continue
		}
		go func() {
			for {
				err := node.managePeerStream(p)
				if err != nil {
					logger.Println("peer error", err)
				}
				time.Sleep(1 * time.Second)
			}
		}()
	}
	return nil
}

func (node *Node) ListenPeers() error {
	err := node.transport.Listen()
	if err != nil {
		return err
	}

	for {
		c, err := node.transport.Accept()
		if err != nil {
			return err
		}
		go func(client network.Client) error {
			defer client.Close()

			peer, err := node.authenticatePeer(client)
			if err != nil {
				logger.Println("peer authentication error", err)
				return err
			}

			for {
				data, err := client.Receive()
				if err != nil {
					return err
				}
				msg, err := parseNetworkMessage(data)
				if err != nil {
					return err
				}
				logger.Println("NODE", msg.Type)
				switch msg.Type {
				case MessageTypePing:
					err = client.Send(buildPongMessage())
					if err != nil {
						return err
					}
				case MessageTypeSnapshot:
					payload := msg.Snapshot.Payload()
					for _, s := range msg.Snapshot.Signatures {
						if peer.Id.PublicSpendKey.Verify(payload, s) {
							node.feedMempool(msg.Snapshot)
							break
						}
					}
				}
			}
		}(c)
	}
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
