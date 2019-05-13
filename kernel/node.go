package kernel

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/network"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/allegro/bigcache"
)

const (
	MempoolSize = 8192
)

type Node struct {
	IdForNetwork    crypto.Hash
	Signer          common.Address
	Graph           *RoundGraph
	TopoCounter     *TopologicalSequence
	SnapshotsPool   map[crypto.Hash][]*crypto.Signature
	SignaturesPool  map[crypto.Hash]*crypto.Signature
	CachePool       map[crypto.Hash][]*common.Snapshot
	signaturesCache *bigcache.BigCache
	Peer            *network.Peer
	SyncPoints      *syncMap
	Listener        string

	ConsensusNodes    map[crypto.Hash]*common.Node
	ConsensusBase     int
	ConsensusPledging *common.Node

	epoch       uint64
	isGenesis   bool
	startAt     time.Time
	networkId   crypto.Hash
	store       storage.Store
	mempoolChan chan *common.Snapshot
	configDir   string
}

func SetupNode(store storage.Store, addr string, dir string) (*Node, error) {
	var node = &Node{
		ConsensusNodes: make(map[crypto.Hash]*common.Node),
		SnapshotsPool:  make(map[crypto.Hash][]*crypto.Signature),
		CachePool:      make(map[crypto.Hash][]*common.Snapshot),
		SignaturesPool: make(map[crypto.Hash]*crypto.Signature),
		SyncPoints:     &syncMap{mutex: new(sync.RWMutex), m: make(map[crypto.Hash]*network.SyncPoint)},
		store:          store,
		mempoolChan:    make(chan *common.Snapshot, MempoolSize),
		configDir:      dir,
		TopoCounter:    getTopologyCounter(store),
		startAt:        time.Now(),
	}

	node.LoadNodeConfig()
	err := node.LoadCacheStorage()
	if err != nil {
		return nil, err
	}

	logger.Println("Validating graph entries...")
	total, invalid, err := node.store.ValidateGraphEntries()
	if err != nil {
		return nil, err
	}
	if invalid > 0 {
		return nil, fmt.Errorf("Validate graph with %d/%d invalid entries\n", invalid, total)
	}
	logger.Printf("Validate graph with %d total entries\n", total)

	err = node.LoadGenesis(dir)
	if err != nil {
		return nil, err
	}

	err = node.LoadConsensusNodes()
	if err != nil {
		return nil, err
	}

	graph, err := LoadRoundGraph(node.store, node.networkId, node.IdForNetwork)
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
	logger.Printf("Signer:\t%s\n", node.Signer.String())
	logger.Printf("Network:\t%s\n", node.networkId.String())
	logger.Printf("Node Id:\t%s\n", node.IdForNetwork.String())
	logger.Printf("Topology:\t%d\n", node.TopoCounter.seq)
	return node, nil
}

func (node *Node) LoadCacheStorage() error {
	c := bigcache.DefaultConfig(config.Custom.CacheTTL * time.Second)
	c.HardMaxCacheSize = config.Custom.MaxCacheSize
	c.Verbose = true
	cache, err := bigcache.NewBigCache(c)
	if err != nil {
		return err
	}
	node.signaturesCache = cache
	return nil
}

func (node *Node) LoadNodeConfig() {
	var addr common.Address
	addr.PrivateSpendKey = config.Custom.Signer
	addr.PublicSpendKey = addr.PrivateSpendKey.Public()
	addr.PrivateViewKey = addr.PublicSpendKey.DeterministicHashDerive()
	addr.PublicViewKey = addr.PrivateViewKey.Public()
	node.Signer = addr
	node.Listener = config.Custom.Listener
}

func (node *Node) LoadConsensusNodes() error {
	node.ConsensusBase = 0
	for _, cn := range node.store.ReadConsensusNodes() {
		logger.Println(cn.Signer.String(), cn.State)
		switch cn.State {
		case common.NodeStatePledging:
			node.ConsensusPledging = cn
			node.ConsensusBase += 1
		case common.NodeStateAccepted:
			idForNetwork := cn.Signer.Hash().ForNetwork(node.networkId)
			node.ConsensusNodes[idForNetwork] = cn
			node.ConsensusBase += 1
		case common.NodeStateDeparting:
			node.ConsensusBase += 1
		}
	}
	return nil
}

func (node *Node) AddNeighborsFromConfig() error {
	f, err := ioutil.ReadFile(node.configDir + "/nodes.json")
	if err != nil {
		return err
	}
	var inputs []struct {
		Signer common.Address `json:"signer"`
		Host   string         `json:"host"`
	}
	err = json.Unmarshal(f, &inputs)
	if err != nil {
		return err
	}
	for _, in := range inputs {
		if in.Signer.String() == node.Signer.String() {
			continue
		}
		id := in.Signer.Hash().ForNetwork(node.networkId)
		if node.ConsensusNodes[id] == nil {
			continue
		}
		node.Peer.AddNeighbor(id, in.Host)
	}

	return nil
}

func (node *Node) ListenNeighbors() error {
	return node.Peer.ListenNeighbors()
}

func (node *Node) NetworkId() crypto.Hash {
	return node.networkId
}

func (node *Node) Uptime() time.Duration {
	return time.Now().Sub(node.startAt)
}

func (node *Node) GetCacheStore() *bigcache.BigCache {
	return node.signaturesCache
}

func (node *Node) BuildGraph() []*network.SyncPoint {
	return node.Graph.FinalCache
}

func (node *Node) BuildAuthenticationMessage() []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(time.Now().Unix()))
	hash := node.Signer.Hash().ForNetwork(node.networkId)
	data = append(data, hash[:]...)
	sig := node.Signer.PrivateSpendKey.Sign(data)
	data = append(data, sig[:]...)
	return append(data, []byte(node.Listener)...)
}

func (node *Node) Authenticate(msg []byte) (crypto.Hash, string, error) {
	ts := binary.BigEndian.Uint64(msg[:8])
	if time.Now().Unix()-int64(ts) > 3 {
		return crypto.Hash{}, "", errors.New("peer authentication message timeout")
	}

	var peerId crypto.Hash
	copy(peerId[:], msg[8:40])
	peer := node.ConsensusNodes[peerId]
	if node.ConsensusPledging != nil && node.ConsensusPledging.Signer.Hash().ForNetwork(node.networkId) == peerId {
		peer = node.ConsensusPledging
	}
	if peer == nil || peerId == node.IdForNetwork {
		return crypto.Hash{}, "", fmt.Errorf("peer authentication invalid consensus peer %s", peerId)
	}

	var sig crypto.Signature
	copy(sig[:], msg[40:40+len(sig)])
	if peer.Signer.PublicSpendKey.Verify(msg[:40], sig) {
		return peerId, string(msg[40+len(sig):]), nil
	}
	return crypto.Hash{}, "", fmt.Errorf("peer authentication message signature invalid %s", peerId)
}

func (node *Node) QueueAppendSnapshot(peerId crypto.Hash, s *common.Snapshot) error {
	s.Hash = s.PayloadHash()
	if len(s.Signatures) != 1 && !node.verifyFinalization(s.Signatures) {
		return node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash, 0)
	}
	inNode, err := node.store.CheckTransactionInNode(s.NodeId, s.Transaction)
	if err != nil {
		return err
	}
	if inNode {
		node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash, 1)
		return node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash, 1)
	}

	sigs := make([]*crypto.Signature, 0)
	signaturesFilter := make(map[string]bool)
	signersMap := make(map[crypto.Hash]bool)
	for _, sig := range s.Signatures {
		if signaturesFilter[sig.String()] {
			continue
		}
		for idForNetwork, cn := range node.ConsensusNodes {
			if signersMap[idForNetwork] {
				continue
			}
			if node.CacheVerify(s.Hash, *sig, cn.Signer.PublicSpendKey) {
				sigs = append(sigs, sig)
				signersMap[idForNetwork] = true
				break
			}
		}
		if n := node.ConsensusPledging; n != nil {
			id := n.Signer.Hash().ForNetwork(node.networkId)
			if id == s.NodeId && s.RoundNumber == 0 && node.CacheVerify(s.Hash, *sig, n.Signer.PublicSpendKey) {
				sigs = append(sigs, sig)
				signersMap[id] = true
			}
		}
		signaturesFilter[sig.String()] = true
	}
	s.Signatures = sigs

	if node.verifyFinalization(s.Signatures) {
		node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash, 1)
		err := node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash, 1)
		if err != nil {
			return err
		}
		return node.store.QueueAppendSnapshot(peerId, s, true)
	}

	err = node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash, 0)
	if err != nil || len(s.Signatures) != 1 {
		return err
	}
	if !signersMap[s.NodeId] && s.NodeId != node.IdForNetwork {
		return nil
	}
	if !signersMap[peerId] {
		return nil
	}
	if !node.CheckCatchUpWithPeers() {
		return nil
	}
	return node.store.QueueAppendSnapshot(peerId, s, false)
}

func (node *Node) SendTransactionToPeer(peerId, hash crypto.Hash) error {
	tx, err := node.store.ReadTransaction(hash)
	if err != nil {
		return err
	}
	if tx == nil {
		tx, err = node.store.CacheGetTransaction(hash)
		if err != nil || tx == nil {
			return err
		}
	}
	return node.Peer.SendTransactionMessage(peerId, tx)
}

func (node *Node) CachePutTransaction(peerId crypto.Hash, tx *common.VersionedTransaction) error {
	node.Peer.ConfirmTransactionForPeer(peerId, tx)
	return node.store.CachePutTransaction(tx)
}

func (node *Node) ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	return node.store.ReadSnapshotsSinceTopology(offset, count)
}

func (node *Node) ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	return node.store.ReadSnapshotsForNodeRound(nodeIdWithNetwork, round)
}

func (node *Node) UpdateSyncPoint(peerId crypto.Hash, points []*network.SyncPoint) {
	if node.ConsensusNodes[peerId] == nil {
		return
	}
	for _, p := range points {
		if p.NodeId == node.IdForNetwork {
			node.SyncPoints.Set(peerId, p)
		}
	}
}

func (node *Node) CheckBroadcastedToPeers() bool {
	count, threshold := 1, len(node.ConsensusNodes)*2/3+1
	final := node.Graph.MyFinalNumber
	for id, _ := range node.ConsensusNodes {
		remote := node.SyncPoints.Get(id)
		if remote == nil {
			continue
		}
		if remote.Number+1 >= final {
			count += 1
		}
	}
	return count >= threshold
}

func (node *Node) CheckCatchUpWithPeers() bool {
	threshold := len(node.ConsensusNodes)*2/3 + 1
	if node.SyncPoints.Len() < threshold {
		return false
	}

	final := node.Graph.MyFinalNumber
	cache := node.Graph.MyCacheRound
	for id, _ := range node.ConsensusNodes {
		remote := node.SyncPoints.Get(id)
		if remote == nil {
			continue
		}
		if remote.Number <= final {
			continue
		}
		if remote.Number > final+1 {
			return false
		}
		if cache == nil {
			return false
		}
		if cache.Hash != remote.Hash {
			return false
		}
		if cache.Start+config.SnapshotRoundGap*100 > uint64(time.Now().UnixNano()) {
			return false
		}
	}
	return true
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

type syncMap struct {
	mutex *sync.RWMutex
	m     map[crypto.Hash]*network.SyncPoint
}

func (s *syncMap) Set(k crypto.Hash, p *network.SyncPoint) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.m[k] = p
}

func (s *syncMap) Get(k crypto.Hash) *network.SyncPoint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.m[k]
}

func (s *syncMap) Len() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.m)
}
