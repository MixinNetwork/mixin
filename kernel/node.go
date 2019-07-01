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
	"github.com/VictoriaMetrics/fastcache"
)

const (
	MempoolSize = 8192
)

type Node struct {
	IdForNetwork   crypto.Hash
	Signer         common.Address
	Graph          *RoundGraph
	TopoCounter    *TopologicalSequence
	SnapshotsPool  map[crypto.Hash][]*crypto.Signature
	SignaturesPool map[crypto.Hash]*crypto.Signature
	CachePool      map[crypto.Hash][]*common.Snapshot
	cacheStore     *fastcache.Cache
	Peer           *network.Peer
	SyncPoints     *syncMap
	Listener       string

	ActiveNodes       map[crypto.Hash]*common.Node
	ConsensusNodes    map[crypto.Hash]*common.Node
	ConsensusPledging *common.Node

	genesisNodesMap map[crypto.Hash]bool
	genesisNodes    []crypto.Hash
	epoch           uint64
	startAt         time.Time
	networkId       crypto.Hash
	persistStore    storage.Store
	mempoolChan     chan *common.Snapshot
	configDir       string
}

func SetupNode(persistStore storage.Store, cacheStore *fastcache.Cache, addr string, dir string) (*Node, error) {
	var node = &Node{
		ConsensusNodes:  make(map[crypto.Hash]*common.Node),
		SnapshotsPool:   make(map[crypto.Hash][]*crypto.Signature),
		CachePool:       make(map[crypto.Hash][]*common.Snapshot),
		SignaturesPool:  make(map[crypto.Hash]*crypto.Signature),
		SyncPoints:      &syncMap{mutex: new(sync.RWMutex), m: make(map[crypto.Hash]*network.SyncPoint)},
		genesisNodesMap: make(map[crypto.Hash]bool),
		persistStore:    persistStore,
		cacheStore:      cacheStore,
		mempoolChan:     make(chan *common.Snapshot, MempoolSize),
		configDir:       dir,
		TopoCounter:     getTopologyCounter(persistStore),
		startAt:         time.Now(),
	}

	node.LoadNodeConfig()

	logger.Println("Validating graph entries...")
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
	logger.Printf("Validate graph with %d total entries\n", total)

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

func (node *Node) ConsensusBase(timestamp uint64) int {
	if timestamp == 0 {
		timestamp = uint64(time.Now().UnixNano())
	}
	if t := node.epoch + config.SnapshotReferenceThreshold*config.SnapshotRoundGap*2; t > timestamp {
		timestamp = t
	}
	consensusBase := 0
	for _, cn := range node.ActiveNodes {
		threshold := config.SnapshotReferenceThreshold * config.SnapshotRoundGap
		if threshold > uint64(3*time.Minute) {
			panic("should never be here")
		}
		if cn.State == common.NodeStatePledging {
			// FIXME the pledge transaction may be broadcasted very late
			// at this situation, the node should be treated as evil
			if config.KernelNodeAcceptPeriodMinimum < time.Hour {
				panic("should never be here")
			}
			threshold = uint64(config.KernelNodeAcceptPeriodMinimum) - threshold*3
		}
		if cn.Timestamp+threshold < timestamp {
			consensusBase++
		}
	}
	if consensusBase < len(node.genesisNodes) {
		panic(fmt.Errorf("invalid consensus base %d %d %d", timestamp, consensusBase, len(node.genesisNodes)))
	}
	return consensusBase
}

func (node *Node) LoadConsensusNodes() error {
	node.ConsensusPledging = nil
	activeNodes := make(map[crypto.Hash]*common.Node)
	consensusNodes := make(map[crypto.Hash]*common.Node)
	for _, cn := range node.persistStore.ReadConsensusNodes() {
		if cn.Timestamp == 0 {
			cn.Timestamp = node.epoch
		}
		idForNetwork := cn.Signer.Hash().ForNetwork(node.networkId)
		logger.Println(idForNetwork, cn.Signer.String(), cn.State, cn.Timestamp)
		switch cn.State {
		case common.NodeStatePledging:
			activeNodes[idForNetwork] = cn
			node.ConsensusPledging = cn
		case common.NodeStateAccepted:
			activeNodes[idForNetwork] = cn
			consensusNodes[idForNetwork] = cn
		case common.NodeStateDeparting:
			activeNodes[idForNetwork] = cn
		}
	}
	node.ActiveNodes = activeNodes
	node.ConsensusNodes = consensusNodes
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

func (node *Node) GetCacheStore() *fastcache.Cache {
	return node.cacheStore
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

func (node *Node) VerifyAndQueueAppendSnapshotdDeprecated(peerId crypto.Hash, s *common.Snapshot) error {
	s.Hash = s.PayloadHash()
	if !node.verifyFinalizationDeprecated(s.Timestamp, s.Signatures) {
		return node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash, 0)
	}
	inNode, err := node.persistStore.CheckTransactionInNode(s.NodeId, s.Transaction)
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
	for i, sig := range s.Signatures {
		s.Signatures[i] = nil
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
	s.Signatures = s.Signatures[:len(sigs)]
	for i := range sigs {
		s.Signatures[i] = sigs[i]
	}

	err = node.Peer.SendSnapshotConfirmMessage(peerId, s.Hash, 0)
	if err != nil {
		return err
	}
	if node.verifyFinalizationDeprecated(s.Timestamp, s.Signatures) {
		node.Peer.ConfirmSnapshotForPeer(peerId, s.Hash, 1)
		return node.QueueAppendSnapshot(peerId, s, true)
	}
	return nil
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
	node.Peer.ConfirmTransactionForPeer(peerId, tx)
	return node.persistStore.CachePutTransaction(tx)
}

func (node *Node) ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	return node.persistStore.ReadSnapshotsSinceTopology(offset, count)
}

func (node *Node) ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	return node.persistStore.ReadSnapshotsForNodeRound(nodeIdWithNetwork, round)
}

func (node *Node) UpdateSyncPoint(peerId crypto.Hash, points []*network.SyncPoint) {
	if node.ConsensusNodes[peerId] == nil { // FIXME concurrent map read write
		return
	}
	for _, p := range points {
		if p.NodeId == node.IdForNetwork {
			node.SyncPoints.Set(peerId, p)
		}
	}
}

func (node *Node) CheckBroadcastedToPeers() bool {
	count, threshold := 1, node.ConsensusBase(0)*2/3+1
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
	threshold := node.ConsensusBase(0)*2/3 + 1
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
		cf := cache.asFinal()
		if cf == nil {
			return false
		}
		if cf.Hash != remote.Hash {
			return false
		}
		if cf.Start+config.SnapshotRoundGap*100 > uint64(time.Now().UnixNano()) {
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
