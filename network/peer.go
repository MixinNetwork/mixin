package network

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/vmihailenco/msgpack"
)

const (
	PeerMessageTypeSnapshot           = 0
	PeerMessageTypePing               = 1
	PeerMessageTypeAuthentication     = 3
	PeerMessageTypeGraph              = 4
	PeerMessageTypeSnapshotConfirm    = 5
	PeerMessageTypeTransactionRequest = 6
	PeerMessageTypeTransaction        = 7
)

type ConfirmMap struct {
	sync.Map
}

func (m *ConfirmMap) Get(peerId, snap crypto.Hash, finalized byte) time.Time {
	hash := snap.ForNetwork(peerId)
	key := crypto.NewHash(append(hash[:], finalized))
	val, _ := m.Load(key)
	ts, _ := val.(time.Time)
	return ts
}

type PeerMessage struct {
	Type            uint8
	Snapshot        *common.Snapshot
	SnapshotHash    crypto.Hash
	Finalized       byte
	Transaction     *common.SignedTransaction
	TransactionHash crypto.Hash
	FinalCache      []*SyncPoint
	Data            []byte
}

type SyncHandle interface {
	BuildAuthenticationMessage() []byte
	Authenticate(msg []byte) (crypto.Hash, error)
	BuildGraph() []*SyncPoint
	QueueAppendSnapshot(peerId crypto.Hash, s *common.Snapshot)
	SendTransactionToPeer(peerId, tx crypto.Hash) error
	CachePutTransaction(tx *common.SignedTransaction) error
	ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error)
}

type SyncPoint struct {
	NodeId crypto.Hash
	Number uint64
}

type Peer struct {
	IdForNetwork crypto.Hash
	Address      string

	snapshotsConfirmations *ConfirmMap
	neighbors              map[crypto.Hash]*Peer
	handle                 SyncHandle
	transport              Transport
	send                   chan []byte
	sync                   chan []*SyncPoint
}

func (me *Peer) AddNeighbor(idForNetwork crypto.Hash, addr string) {
	peer := NewPeer(nil, idForNetwork, addr)
	if peer.Address == me.Address || me.neighbors[peer.IdForNetwork] != nil {
		return
	}
	me.neighbors[peer.IdForNetwork] = peer

	go me.openPeerStreamLoop(peer)
	go me.syncToNeighborLoop(peer)
}

func NewPeer(handle SyncHandle, idForNetwork crypto.Hash, addr string) *Peer {
	return &Peer{
		IdForNetwork:           idForNetwork,
		Address:                addr,
		snapshotsConfirmations: new(ConfirmMap),
		neighbors:              make(map[crypto.Hash]*Peer),
		send:                   make(chan []byte, 8192),
		sync:                   make(chan []*SyncPoint),
		handle:                 handle,
	}
}

func (me *Peer) SendTransactionRequestMessage(idForNetwork crypto.Hash, tx crypto.Hash) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	for _, p := range me.neighbors {
		if p.IdForNetwork == idForNetwork {
			return p.SendData(buildTransactionRequestMessage(tx))
		}
	}
	return nil
}

func (me *Peer) SendTransactionMessage(idForNetwork crypto.Hash, tx *common.SignedTransaction) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	for _, p := range me.neighbors {
		if p.IdForNetwork == idForNetwork {
			return p.SendData(buildTransactionMessage(tx))
		}
	}
	return nil
}

func (me *Peer) SendSnapshotConfirmMessage(idForNetwork crypto.Hash, snap crypto.Hash, finalized byte) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	for _, p := range me.neighbors {
		if p.IdForNetwork == idForNetwork {
			return p.SendData(buildSnapshotConfirmMessage(snap, finalized))
		}
	}
	return nil
}

func (me *Peer) ConfirmSnapshotForPeer(idForNetwork, snap crypto.Hash, finalized byte) error {
	hash := snap.ForNetwork(idForNetwork)
	key := crypto.NewHash(append(hash[:], finalized))
	me.snapshotsConfirmations.Store(key, time.Now())
	return nil
}

func (me *Peer) SendSnapshotMessage(idForNetwork crypto.Hash, s *common.Snapshot, finalized byte) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	confirmTime := me.snapshotsConfirmations.Get(idForNetwork, s.PayloadHash(), finalized)
	if confirmTime.Add(config.CacheTTL / 2).After(time.Now()) {
		return nil
	}
	for _, p := range me.neighbors {
		if p.IdForNetwork == idForNetwork {
			return p.SendData(buildSnapshotMessage(s))
		}
	}
	return nil
}

func (p *Peer) SendData(data []byte) error {
	select {
	case p.send <- data:
		return nil
	case <-time.After(1 * time.Second):
		return errors.New("peer send timeout")
	}
}

func (me *Peer) ListenNeighbors() error {
	transport, err := NewTcpServer(me.Address)
	if err != nil {
		return err
	}
	me.transport = transport

	err = me.transport.Listen()
	if err != nil {
		return err
	}

	for {
		c, err := me.transport.Accept()
		if err != nil {
			return err
		}
		go func(c Client) {
			err := me.acceptNeighborConnection(c)
			if err != nil {
				logger.Println("accept neighbor error", err)
			}
		}(c)
	}
}

func parseNetworkMessage(data []byte) (*PeerMessage, error) {
	if len(data) < 1 {
		return nil, errors.New("invalid message data")
	}
	msg := &PeerMessage{Type: data[0]}
	switch msg.Type {
	case PeerMessageTypeSnapshot:
		var ss common.Snapshot
		err := msgpack.Unmarshal(data[1:], &ss)
		if err != nil {
			return nil, err
		}
		msg.Snapshot = &ss
	case PeerMessageTypeGraph:
		err := msgpack.Unmarshal(data[1:], &msg.FinalCache)
		if err != nil {
			return nil, err
		}
	case PeerMessageTypePing:
	case PeerMessageTypeAuthentication:
		msg.Data = data[1:]
	case PeerMessageTypeSnapshotConfirm:
		msg.Finalized = data[1]
		copy(msg.SnapshotHash[:], data[2:])
	case PeerMessageTypeTransaction:
		var tx common.SignedTransaction
		err := msgpack.Unmarshal(data[1:], &tx)
		if err != nil {
			return nil, err
		}
		msg.Transaction = &tx
	case PeerMessageTypeTransactionRequest:
		copy(msg.TransactionHash[:], data[1:])
	}
	return msg, nil
}

func buildAuthenticationMessage(data []byte) []byte {
	header := []byte{PeerMessageTypeAuthentication}
	return append(header, data...)
}

func buildPingMessage() []byte {
	return []byte{PeerMessageTypePing}
}

func buildSnapshotMessage(ss *common.Snapshot) []byte {
	data := common.MsgpackMarshalPanic(ss)
	return append([]byte{PeerMessageTypeSnapshot}, data...)
}

func buildSnapshotConfirmMessage(snap crypto.Hash, finalized byte) []byte {
	return append([]byte{PeerMessageTypeSnapshotConfirm, finalized}, snap[:]...)
}

func buildTransactionMessage(tx *common.SignedTransaction) []byte {
	data := common.MsgpackMarshalPanic(tx)
	return append([]byte{PeerMessageTypeTransaction}, data...)
}

func buildTransactionRequestMessage(tx crypto.Hash) []byte {
	return append([]byte{PeerMessageTypeTransactionRequest}, tx[:]...)
}

func buildGraphMessage(points []*SyncPoint) []byte {
	data := common.MsgpackMarshalPanic(points)
	return append([]byte{PeerMessageTypeGraph}, data...)
}

func (me *Peer) openPeerStreamLoop(p *Peer) {
	for {
		err := me.openPeerStream(p)
		if err != nil {
			logger.Println("neighbor open stream error", err)
		}
		time.Sleep(1 * time.Second)
	}
}

func (me *Peer) openPeerStream(peer *Peer) error {
	logger.Println("OPEN PEER STREAM", peer.Address)
	transport, err := NewTcpClient(peer.Address)
	if err != nil {
		return err
	}
	client, err := transport.Dial()
	if err != nil {
		return err
	}
	defer client.Close()
	logger.Println("DIAL PEER STREAM", peer.Address)

	err = client.Send(buildAuthenticationMessage(me.handle.BuildAuthenticationMessage()))
	if err != nil {
		return err
	}
	logger.Println("AUTH PEER STREAM", peer.Address)

	pingTicker := time.NewTicker(1 * time.Second)
	defer pingTicker.Stop()

	graphTicker := time.NewTicker(time.Duration(config.SnapshotRoundGap))
	defer graphTicker.Stop()

	logger.Println("LOOP PEER STREAM", peer.Address)
	for {
		select {
		case msg := <-peer.send:
			err := client.Send(msg)
			if err != nil {
				return err
			}
		case <-graphTicker.C:
			err := client.Send(buildGraphMessage(me.handle.BuildGraph()))
			if err != nil {
				return err
			}
		case <-pingTicker.C:
			err := client.Send(buildPingMessage())
			if err != nil {
				return err
			}
		}
	}
}

func (me *Peer) acceptNeighborConnection(client Client) error {
	defer client.Close()

	peer, err := me.authenticateNeighbor(client)
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
		switch msg.Type {
		case PeerMessageTypePing:
		case PeerMessageTypeSnapshot:
			me.handle.QueueAppendSnapshot(peer.IdForNetwork, msg.Snapshot)
		case PeerMessageTypeGraph:
			peer.sync <- msg.FinalCache
		case PeerMessageTypeTransactionRequest:
			me.handle.SendTransactionToPeer(peer.IdForNetwork, msg.TransactionHash)
		case PeerMessageTypeTransaction:
			me.handle.CachePutTransaction(msg.Transaction)
		case PeerMessageTypeSnapshotConfirm:
			me.ConfirmSnapshotForPeer(peer.IdForNetwork, msg.SnapshotHash, msg.Finalized)
		}
	}
}

func (me *Peer) authenticateNeighbor(client Client) (*Peer, error) {
	var peer *Peer
	auth := make(chan error)
	go func() {
		data, err := client.Receive()
		if err != nil {
			auth <- err
			return
		}
		msg, err := parseNetworkMessage(data)
		if err != nil {
			auth <- err
			return
		}
		if msg.Type != PeerMessageTypeAuthentication {
			auth <- errors.New("peer authentication invalid message type")
			return
		}

		id, err := me.handle.Authenticate(msg.Data)
		if err != nil {
			auth <- err
			return
		}
		for _, p := range me.neighbors {
			if id != p.IdForNetwork {
				continue
			}
			peer = p
			auth <- nil
			return
		}
		auth <- errors.New("peer authentication message signature invalid")
	}()

	select {
	case err := <-auth:
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("peer authentication failed %s", err.Error())
		}
	case <-time.After(3 * time.Second):
		client.Close()
		return nil, errors.New("peer authentication timeout")
	}
	return peer, nil
}
