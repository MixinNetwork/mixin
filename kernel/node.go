package kernel

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/network"
	"github.com/MixinNetwork/mixin/storage"
	"github.com/VictoriaMetrics/fastcache"
)

const (
	MempoolSize = 8192
)

type Node struct {
	IdForNetwork crypto.Hash
	Signer       common.Address
	Graph        *RoundGraph
	TopoCounter  *TopologicalSequence
	cacheStore   *fastcache.Cache
	Peer         *network.Peer
	SyncPoints   *syncMap
	Listener     string

	ActiveNodes          []*common.Node
	ConsensusNodes       map[crypto.Hash]*common.Node
	SortedConsensusNodes []crypto.Hash
	ConsensusIndex       int
	ConsensusPledging    *common.Node
	ConsensusRemoved     *common.Node

	CosiAggregators *aggregatorMap
	CosiVerifiers   map[crypto.Hash]*CosiVerifier

	genesisNodesMap map[crypto.Hash]bool
	genesisNodes    []crypto.Hash
	Epoch           uint64
	startAt         time.Time
	networkId       crypto.Hash
	persistStore    storage.Store
	cosiActionsChan chan *CosiAction
	configDir       string
}

func SetupNode(persistStore storage.Store, cacheStore *fastcache.Cache, addr string, dir string) (*Node, error) {
	var node = &Node{
		SyncPoints:      &syncMap{mutex: new(sync.RWMutex), m: make(map[crypto.Hash]*network.SyncPoint)},
		ConsensusIndex:  -1,
		CosiAggregators: &aggregatorMap{mutex: new(sync.RWMutex), m: make(map[crypto.Hash]*CosiAggregator)},
		CosiVerifiers:   make(map[crypto.Hash]*CosiVerifier),
		genesisNodesMap: make(map[crypto.Hash]bool),
		persistStore:    persistStore,
		cacheStore:      cacheStore,
		cosiActionsChan: make(chan *CosiAction, MempoolSize),
		configDir:       dir,
		TopoCounter:     getTopologyCounter(persistStore),
		startAt:         clock.Now(),
	}

	node.LoadNodeConfig()

	logger.Println("Validating graph entries...")
	start := clock.Now()
	var state struct{ Id crypto.Hash }
	_, err := node.persistStore.StateGet("network", &state)
	if err != nil {
		return nil, err
	}
	total, invalid, err := node.persistStore.ValidateGraphEntries(state.Id)
	if err != nil {
		return nil, err
	}
	if invalid > 0 {
		return nil, fmt.Errorf("Validate graph with %d/%d invalid entries\n", invalid, total)
	}
	logger.Printf("Validate graph with %d total entries in %s\n", total, clock.Now().Sub(start).String())

	err = node.LoadGenesis(dir)
	if err != nil {
		return nil, err
	}

	err = node.LoadConsensusNodes()
	if err != nil {
		return nil, err
	}

	graph, err := LoadRoundGraph(node.persistStore, node.networkId, node.IdForNetwork)
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

func (node *Node) LoadNodeConfig() {
	var addr common.Address
	addr.PrivateSpendKey = config.Custom.Signer
	addr.PublicSpendKey = addr.PrivateSpendKey.Public()
	addr.PrivateViewKey = addr.PublicSpendKey.DeterministicHashDerive()
	addr.PublicViewKey = addr.PrivateViewKey.Public()
	node.Signer = addr
	node.Listener = config.Custom.Listener
}

func (node *Node) ConsensusKeys(timestamp uint64) []*crypto.Key {
	if timestamp == 0 {
		timestamp = uint64(clock.Now().UnixNano())
	}

	var keys []*crypto.Key
	for _, cn := range node.ActiveNodes {
		if cn.State != common.NodeStateAccepted {
			continue
		}
		if node.genesisNodesMap[cn.IdForNetwork(node.networkId)] || cn.Timestamp+uint64(config.KernelNodeAcceptPeriodMinimum) < timestamp {
			keys = append(keys, &cn.Signer.PublicSpendKey)
		}
	}
	return keys
}

func (node *Node) ConsensusThreshold(timestamp uint64) int {
	if timestamp == 0 {
		timestamp = uint64(clock.Now().UnixNano())
	}
	consensusBase := 0
	for _, cn := range node.ActiveNodes {
		threshold := config.SnapshotReferenceThreshold * config.SnapshotRoundGap
		if threshold > uint64(3*time.Minute) {
			panic("should never be here")
		}
		switch cn.State {
		case common.NodeStatePledging:
			// FIXME the pledge transaction may be broadcasted very late
			// at this situation, the node should be treated as evil
			if config.KernelNodeAcceptPeriodMinimum < time.Hour {
				panic("should never be here")
			}
			threshold = uint64(config.KernelNodeAcceptPeriodMinimum) - threshold*3
			if cn.Timestamp+threshold < timestamp {
				consensusBase++
			}
		case common.NodeStateAccepted:
			if node.genesisNodesMap[cn.IdForNetwork(node.networkId)] || cn.Timestamp+threshold < timestamp {
				consensusBase++
			}
		case common.NodeStateResigning:
			consensusBase++
		}
	}
	if consensusBase < len(node.genesisNodes) {
		logger.Printf("invalid consensus base %d %d %d\n", timestamp, consensusBase, len(node.genesisNodes))
		return 1000
	}
	return consensusBase*2/3 + 1
}

func (node *Node) LoadConsensusNodes() error {
	node.ConsensusPledging = nil
	node.ConsensusRemoved = nil
	activeNodes := make([]*common.Node, 0)
	consensusNodes := make(map[crypto.Hash]*common.Node)
	sortedConsensusNodes := make([]crypto.Hash, 0)
	for _, cn := range node.SortAllNodesByTimestampAndId() {
		if cn.Timestamp == 0 {
			cn.Timestamp = node.Epoch
		}
		idForNetwork := cn.IdForNetwork(node.networkId)
		logger.Println(idForNetwork, cn.Signer.String(), cn.State, cn.Timestamp)
		switch cn.State {
		case common.NodeStatePledging:
			node.ConsensusPledging = cn
			activeNodes = append(activeNodes, cn)
		case common.NodeStateAccepted:
			consensusNodes[idForNetwork] = cn
			activeNodes = append(activeNodes, cn)
		case common.NodeStateResigning:
			activeNodes = append(activeNodes, cn)
		case common.NodeStateRemoved:
			if node.ConsensusRemoved == nil || cn.Timestamp > node.ConsensusRemoved.Timestamp {
				node.ConsensusRemoved = cn
			}
		}
	}
	for _, n := range activeNodes {
		if n.State == common.NodeStateAccepted {
			id := n.IdForNetwork(node.networkId)
			sortedConsensusNodes = append(sortedConsensusNodes, id)
		}
	}
	node.ActiveNodes = activeNodes
	node.ConsensusNodes = consensusNodes
	node.SortedConsensusNodes = sortedConsensusNodes
	for i, id := range node.SortedConsensusNodes {
		if id == node.IdForNetwork {
			node.ConsensusIndex = i
		}
	}
	return nil
}

func (node *Node) ConsensusRemovedRecently(timestamp uint64) *common.Node {
	if timestamp == 0 || node.ConsensusRemoved == nil {
		return nil
	}
	threshold := uint64(config.KernelNodeAcceptPeriodMinimum) / 2
	if node.ConsensusRemoved.Timestamp+threshold > timestamp {
		return node.ConsensusRemoved
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
	return clock.Now().Sub(node.startAt)
}

func (node *Node) GetCacheStore() *fastcache.Cache {
	return node.cacheStore
}

func (node *Node) BuildGraph() []*network.SyncPoint {
	return node.Graph.FinalCache
}

func (node *Node) BuildAuthenticationMessage() []byte {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(clock.Now().Unix()))
	data = append(data, node.Signer.PublicSpendKey[:]...)
	sig := node.Signer.PrivateSpendKey.Sign(data)
	data = append(data, sig[:]...)
	return append(data, []byte(node.Listener)...)
}

func (node *Node) Authenticate(msg []byte) (crypto.Hash, string, error) {
	if len(msg) < 8+len(crypto.Hash{})+len(crypto.Signature{}) {
		return crypto.Hash{}, "", fmt.Errorf("peer authentication message malformated %d", len(msg))
	}
	ts := binary.BigEndian.Uint64(msg[:8])
	if clock.Now().Unix()-int64(ts) > 3 {
		return crypto.Hash{}, "", fmt.Errorf("peer authentication message timeout %d %d", ts, clock.Now().Unix())
	}

	var signer common.Address
	copy(signer.PublicSpendKey[:], msg[8:40])
	signer.PublicViewKey = signer.PublicSpendKey.DeterministicHashDerive().Public()
	peerId := signer.Hash().ForNetwork(node.networkId)
	if peerId == node.IdForNetwork {
		return crypto.Hash{}, "", fmt.Errorf("peer authentication invalid consensus peer %s", peerId)
	}
	peer := node.getPeerConsensusNode(peerId)

	if config.Custom.ConsensusOnly && peer == nil {
		return crypto.Hash{}, "", fmt.Errorf("peer authentication invalid consensus peer %s", peerId)
	}
	if peer.Signer.Hash() != signer.Hash() {
		return crypto.Hash{}, "", fmt.Errorf("peer authentication invalid consensus peer %s", peerId)
	}

	var sig crypto.Signature
	copy(sig[:], msg[40:40+len(sig)])
	if !signer.PublicSpendKey.Verify(msg[:40], sig) {
		return crypto.Hash{}, "", fmt.Errorf("peer authentication message signature invalid %s", peerId)
	}

	listener := string(msg[40+len(sig):])
	return peerId, listener, nil
}

func (node *Node) QueueAppendSnapshot(peerId crypto.Hash, s *common.Snapshot, final bool) error {
	if !final && node.Graph.MyCacheRound == nil {
		return nil
	}
	return node.persistStore.QueueAppendSnapshot(peerId, s, final)
}

func (node *Node) SendTransactionToPeer(peerId, hash crypto.Hash) error {
	tx, _, err := node.persistStore.ReadTransaction(hash)
	if err != nil {
		return err
	}
	if tx == nil {
		tx, err = node.persistStore.CacheGetTransaction(hash)
		if err != nil || tx == nil {
			return err
		}
	}
	return node.Peer.SendTransactionMessage(peerId, tx)
}

func (node *Node) CachePutTransaction(peerId crypto.Hash, tx *common.VersionedTransaction) error {
	return node.persistStore.CachePutTransaction(tx)
}

func (node *Node) ReadAllNodes() []crypto.Hash {
	nodes := node.persistStore.ReadAllNodes()
	hashes := make([]crypto.Hash, len(nodes))
	for i, n := range nodes {
		hashes[i] = n.IdForNetwork(node.networkId)
	}
	return hashes
}

func (node *Node) ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	return node.persistStore.ReadSnapshotsSinceTopology(offset, count)
}

func (node *Node) ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	return node.persistStore.ReadSnapshotsForNodeRound(nodeIdWithNetwork, round)
}

func (node *Node) UpdateSyncPoint(peerId crypto.Hash, points []*network.SyncPoint) {
	for _, p := range points {
		if p.NodeId == node.IdForNetwork {
			node.SyncPoints.Set(peerId, p)
		}
	}
}

func (node *Node) CheckBroadcastedToPeers() bool {
	count, threshold := 1, node.ConsensusThreshold(0)
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
	final := node.Graph.MyFinalNumber
	cache := node.Graph.MyCacheRound
	updated, threshold := 0, node.ConsensusThreshold(0)

	for id, _ := range node.ConsensusNodes {
		remote := node.SyncPoints.Get(id)
		if remote == nil {
			continue
		}
		updated = updated + 1
		if remote.Number <= final {
			continue
		}
		if remote.Number > final+1 {
			logger.Verbosef("CheckCatchUpWithPeers local(%d)+1 < remote(%s:%d)\n", final, id, remote.Number)
			return false
		}
		if cache == nil {
			logger.Verbosef("CheckCatchUpWithPeers local cache nil\n")
			return false
		}
		cf := cache.asFinal()
		if cf == nil {
			logger.Verbosef("CheckCatchUpWithPeers local cache empty\n")
			return false
		}
		if cf.Hash != remote.Hash {
			logger.Verbosef("CheckCatchUpWithPeers local(%s) != remote(%s)\n", cf.Hash, remote.Hash)
			return false
		}
		if now := uint64(clock.Now().UnixNano()); cf.Start+config.SnapshotRoundGap*100 > now {
			logger.Verbosef("CheckCatchUpWithPeers local start(%d)+%d > now(%d)\n", cf.Start, config.SnapshotRoundGap*100, now)
			return false
		}
	}

	if updated < threshold {
		logger.Verbosef("CheckCatchUpWithPeers updated(%d) < threshold(%d)\n", updated, threshold)
	}
	return updated >= threshold
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
