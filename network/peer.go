package network

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/patrickmn/go-cache"
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

func (m *ConfirmMap) Exist(key crypto.Hash, duration time.Duration) bool {
	val, _ := m.Load(key)
	ts, _ := val.(time.Time)
	return ts.Add(duration).After(time.Now())
}

type PeerMessage struct {
	Type            uint8
	Snapshot        *common.Snapshot
	SnapshotHash    crypto.Hash
	Finalized       byte
	Transaction     *common.VersionedTransaction
	TransactionHash crypto.Hash
	FinalCache      []*SyncPoint
	Data            []byte
}

type SyncHandle interface {
	BuildAuthenticationMessage() []byte
	Authenticate(msg []byte) (crypto.Hash, string, error)
	BuildGraph() []*SyncPoint
	QueueAppendSnapshot(peerId crypto.Hash, s *common.Snapshot) error
	SendTransactionToPeer(peerId, tx crypto.Hash) error
	CachePutTransaction(peerId crypto.Hash, ver *common.VersionedTransaction) error
	ReadSnapshotsSinceTopology(offset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	ReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.SnapshotWithTopologicalOrder, error)
	UpdateSyncPoint(peerId crypto.Hash, points []*SyncPoint)
}

type SyncPoint struct {
	NodeId crypto.Hash
	Number uint64
	Hash   crypto.Hash
}

type ChanMsg struct {
	key  crypto.Hash
	data []byte
}

type Peer struct {
	IdForNetwork crypto.Hash
	Address      string

	storeCache             *cache.Cache
	snapshotsConfirmations *ConfirmMap
	snapshotsCaches        *ConfirmMap
	neighbors              map[crypto.Hash]*Peer
	handle                 SyncHandle
	transport              Transport
	high                   chan *ChanMsg
	normal                 chan *ChanMsg
	sync                   chan []*SyncPoint
	closing                bool
}

func (me *Peer) AddNeighbor(idForNetwork crypto.Hash, addr string) {
	if a, err := net.ResolveUDPAddr("udp", addr); err != nil {
		logger.Println("invalid address", addr, err)
		return
	} else if a.Port < 80 || a.IP == nil {
		logger.Println("invalid address", addr, a.Port, a.IP)
		return
	}
	old := me.neighbors[idForNetwork]
	if old == nil {
		old = NewPeer(nil, idForNetwork, addr)
	} else if old.Address != addr {
		old.closing = true
	} else {
		return
	}
	peer := *old
	peer.closing = false
	me.neighbors[idForNetwork] = &peer

	go me.openPeerStreamLoop(&peer)
	go me.syncToNeighborLoop(&peer)
}

func NewPeer(handle SyncHandle, idForNetwork crypto.Hash, addr string) *Peer {
	return &Peer{
		IdForNetwork:           idForNetwork,
		Address:                addr,
		storeCache:             cache.New(config.CacheTTL, 10*time.Minute),
		snapshotsConfirmations: new(ConfirmMap),
		snapshotsCaches:        new(ConfirmMap),
		neighbors:              make(map[crypto.Hash]*Peer),
		high:                   make(chan *ChanMsg, 1024*1024),
		normal:                 make(chan *ChanMsg, 1024*1024),
		sync:                   make(chan []*SyncPoint),
		handle:                 handle,
	}
}

func (me *Peer) SendTransactionRequestMessage(idForNetwork crypto.Hash, tx crypto.Hash) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}

	key := tx.ForNetwork(idForNetwork)
	key = crypto.NewHash(append(key[:], 'R', 'Q'))
	if me.snapshotsCaches.Exist(key, time.Minute) {
		return nil
	}

	peer := me.neighbors[idForNetwork]
	if peer == nil {
		return nil
	}
	return peer.SendHigh(key, buildTransactionRequestMessage(tx))
}

func (me *Peer) ConfirmTransactionForPeer(idForNetwork crypto.Hash, ver *common.VersionedTransaction) {
	key := ver.PayloadHash().ForNetwork(idForNetwork)
	key = crypto.NewHash(append(key[:], 'P', 'L'))
	me.snapshotsCaches.Store(key, time.Now())
}

func (me *Peer) SendTransactionMessage(idForNetwork crypto.Hash, ver *common.VersionedTransaction) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}

	key := ver.PayloadHash().ForNetwork(idForNetwork)
	key = crypto.NewHash(append(key[:], 'P', 'L'))
	if me.snapshotsCaches.Exist(key, time.Minute) {
		return nil
	}

	peer := me.neighbors[idForNetwork]
	if peer == nil {
		return nil
	}
	return peer.SendHigh(key, buildTransactionMessage(ver))
}

func (me *Peer) SendSnapshotConfirmMessage(idForNetwork crypto.Hash, snap crypto.Hash, finalized byte) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}

	key := snap.ForNetwork(idForNetwork)
	key = crypto.NewHash(append(key[:], finalized, 'C', 'F'))
	if me.snapshotsCaches.Exist(key, time.Minute) {
		return nil
	}

	peer := me.neighbors[idForNetwork]
	if peer == nil {
		return nil
	}
	return peer.SendHigh(key, buildSnapshotConfirmMessage(snap, finalized))
}

func (me *Peer) ConfirmSnapshotForPeer(idForNetwork, snap crypto.Hash, finalized byte) {
	hash := snap.ForNetwork(idForNetwork)
	key := crypto.NewHash(append(hash[:], finalized))
	me.snapshotsConfirmations.Store(key, time.Now())
}

func (me *Peer) SendSnapshotMessage(idForNetwork crypto.Hash, s *common.Snapshot, finalized byte) error {
	if idForNetwork == me.IdForNetwork || len(s.Signatures) == 0 {
		return nil
	}

	hash := s.PayloadHash().ForNetwork(idForNetwork)
	key := crypto.NewHash(append(hash[:], finalized))
	if me.snapshotsConfirmations.Exist(key, config.CacheTTL/2) {
		return nil
	}
	if me.snapshotsCaches.Exist(key, time.Minute) {
		return nil
	}

	peer := me.neighbors[idForNetwork]
	if peer == nil {
		return nil
	}
	return peer.SendNormal(key, buildSnapshotMessage(s))
}

func (p *Peer) SendHigh(key crypto.Hash, data []byte) error {
	select {
	case p.high <- &ChanMsg{key, data}:
		return nil
	case <-time.After(1 * time.Second):
		return errors.New("peer send high timeout")
	}
}

func (p *Peer) SendNormal(key crypto.Hash, data []byte) error {
	select {
	case p.normal <- &ChanMsg{key, data}:
		return nil
	case <-time.After(1 * time.Second):
		return errors.New("peer send normal timeout")
	}
}

func (me *Peer) ListenNeighbors() error {
	transport, err := NewQuicServer(me.Address)
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
		err := common.MsgpackUnmarshal(data[1:], &ss)
		if err != nil {
			return nil, err
		}
		msg.Snapshot = &ss
	case PeerMessageTypeGraph:
		err := common.MsgpackUnmarshal(data[1:], &msg.FinalCache)
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
		ver, err := common.UnmarshalVersionedTransaction(data[1:])
		if err != nil {
			return nil, err
		}
		msg.Transaction = ver
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

func buildTransactionMessage(ver *common.VersionedTransaction) []byte {
	data := ver.Marshal()
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
	var resend *ChanMsg
	for !p.closing {
		msg, err := me.openPeerStream(p, resend)
		if err != nil {
			logger.Println("neighbor open stream error", err)
		}
		resend = msg
		time.Sleep(1 * time.Second)
	}
}

func (me *Peer) openPeerStream(peer *Peer, resend *ChanMsg) (*ChanMsg, error) {
	logger.Println("OPEN PEER STREAM", peer.Address)
	transport, err := NewQuicClient(peer.Address)
	if err != nil {
		return nil, err
	}
	client, err := transport.Dial()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	logger.Println("DIAL PEER STREAM", peer.Address)

	err = client.Send(buildAuthenticationMessage(me.handle.BuildAuthenticationMessage()))
	if err != nil {
		return nil, err
	}
	logger.Println("AUTH PEER STREAM", peer.Address)

	pingTicker := time.NewTicker(1 * time.Second)
	defer pingTicker.Stop()

	graphTicker := time.NewTicker(time.Duration(config.SnapshotRoundGap / 2))
	defer graphTicker.Stop()

	if resend != nil {
		logger.Println("RESEND PEER STREAM", resend.key.String())
		if !me.snapshotsCaches.Exist(resend.key, time.Minute) {
			err := client.Send(resend.data)
			if err != nil {
				return resend, err
			}
			me.snapshotsCaches.Store(resend.key, time.Now())
		}
	}

	logger.Println("LOOP PEER STREAM", peer.Address)
	for {
		if peer.closing {
			return nil, fmt.Errorf("PEER DONE")
		}

		hd, nd := false, false
		select {
		case msg := <-peer.high:
			if !me.snapshotsCaches.Exist(msg.key, time.Minute) {
				err := client.Send(msg.data)
				if err != nil {
					return msg, err
				}
				me.snapshotsCaches.Store(msg.key, time.Now())
			}
		default:
			hd = true
		}

		select {
		case msg := <-peer.normal:
			if !me.snapshotsCaches.Exist(msg.key, time.Minute) {
				err := client.Send(msg.data)
				if err != nil {
					return msg, err
				}
				me.snapshotsCaches.Store(msg.key, time.Now())
			}
		case <-graphTicker.C:
			err := client.Send(buildGraphMessage(me.handle.BuildGraph()))
			if err != nil {
				return nil, err
			}
		case <-pingTicker.C:
			err := client.Send(buildPingMessage())
			if err != nil {
				return nil, err
			}
		default:
			nd = true
		}

		if hd && nd {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (me *Peer) acceptNeighborConnection(client Client) error {
	done := make(chan bool, 1)
	receive := make(chan *PeerMessage, 1024*1024)

	defer func() {
		client.Close()
		done <- true
	}()

	peer, err := me.authenticateNeighbor(client)
	if err != nil {
		logger.Println("peer authentication error", err)
		return err
	}

	go me.handlePeerMessage(peer, receive, done)

	for {
		data, err := client.Receive()
		if err != nil {
			return err
		}
		msg, err := parseNetworkMessage(data)
		if err != nil {
			return err
		}
		select {
		case receive <- msg:
		case <-time.After(1 * time.Second):
			return errors.New("peer receive timeout")
		}
	}
}

func (me *Peer) handlePeerMessage(peer *Peer, receive chan *PeerMessage, done chan bool) {
	for {
		select {
		case <-done:
			return
		case msg := <-receive:
			switch msg.Type {
			case PeerMessageTypePing:
			case PeerMessageTypeSnapshot:
				me.handle.QueueAppendSnapshot(peer.IdForNetwork, msg.Snapshot)
			case PeerMessageTypeGraph:
				me.handle.UpdateSyncPoint(peer.IdForNetwork, msg.FinalCache)
				peer.sync <- msg.FinalCache
			case PeerMessageTypeTransactionRequest:
				me.handle.SendTransactionToPeer(peer.IdForNetwork, msg.TransactionHash)
			case PeerMessageTypeTransaction:
				me.handle.CachePutTransaction(peer.IdForNetwork, msg.Transaction)
			case PeerMessageTypeSnapshotConfirm:
				me.ConfirmSnapshotForPeer(peer.IdForNetwork, msg.SnapshotHash, msg.Finalized)
			}
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

		id, addr, err := me.handle.Authenticate(msg.Data)
		if err != nil {
			auth <- err
			return
		}
		for _, p := range me.neighbors {
			if id != p.IdForNetwork {
				continue
			}
			me.AddNeighbor(id, addr)
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
