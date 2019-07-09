package network

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/VictoriaMetrics/fastcache"
)

const (
	PeerMessageTypePing               = 1
	PeerMessageTypeAuthentication     = 3
	PeerMessageTypeGraph              = 4
	PeerMessageTypeSnapshotConfirm    = 5
	PeerMessageTypeTransactionRequest = 6
	PeerMessageTypeTransaction        = 7

	PeerMessageTypeSnapshotAnnoucement  = 10 // leader send snapshot to peer
	PeerMessageTypeSnapshotCommitment   = 11 // peer generate ri based, send Ri to leader
	PeerMessageTypeTransactionChallenge = 12 // leader send bitmask Z and aggragated R to peer
	PeerMessageTypeSnapshotResponse     = 13 // peer generate A from nodes and Z, send response si = ri + H(R || A || M)ai to leader
	PeerMessageTypeSnapshotFinalization = 14 // leader generate A, verify si B = ri B + H(R || A || M)ai B = Ri + H(R || A || M)Ai, then finaliz based on threshold
)

type PeerMessage struct {
	Type            uint8
	Snapshot        *common.Snapshot
	SnapshotHash    crypto.Hash
	Transaction     *common.VersionedTransaction
	TransactionHash crypto.Hash
	FinalCache      []*SyncPoint
	Data            []byte
}

type SyncHandle interface {
	GetCacheStore() *fastcache.Cache
	BuildAuthenticationMessage() []byte
	Authenticate(msg []byte) (crypto.Hash, string, error)
	BuildGraph() []*SyncPoint
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

	storeCache      *fastcache.Cache
	snapshotsCaches *confirmMap
	neighbors       map[crypto.Hash]*Peer
	handle          SyncHandle
	transport       Transport
	high            chan *ChanMsg
	normal          chan *ChanMsg
	sync            chan []*SyncPoint
	closing         bool
}

func (me *Peer) AddNeighbor(idForNetwork crypto.Hash, addr string) (*Peer, error) {
	if a, err := net.ResolveUDPAddr("udp", addr); err != nil {
		return nil, fmt.Errorf("invalid address %s %s", addr, err)
	} else if a.Port < 80 || a.IP == nil {
		return nil, fmt.Errorf("invalid address %s %d %s", addr, a.Port, a.IP)
	}
	old := me.neighbors[idForNetwork]
	if old != nil && old.Address == addr {
		return old, nil
	} else if old != nil {
		old.closing = true
	}

	peer := NewPeer(nil, idForNetwork, addr)
	me.neighbors[idForNetwork] = peer
	go me.openPeerStreamLoop(peer)
	go me.syncToNeighborLoop(peer)
	return peer, nil
}

func NewPeer(handle SyncHandle, idForNetwork crypto.Hash, addr string) *Peer {
	peer := &Peer{
		IdForNetwork: idForNetwork,
		Address:      addr,
		neighbors:    make(map[crypto.Hash]*Peer),
		high:         make(chan *ChanMsg, 1024*1024),
		normal:       make(chan *ChanMsg, 1024*1024),
		sync:         make(chan []*SyncPoint),
		handle:       handle,
	}
	if handle != nil {
		peer.storeCache = handle.GetCacheStore()
		peer.snapshotsCaches = &confirmMap{cache: peer.storeCache}
	}
	return peer
}

func (me *Peer) SendSnapshotAnnouncementMessage(idForNetwork crypto.Hash, s *common.Snapshot) error {
	data := buildSnapshotAnnouncementMessage(s)
	return me.sendSnapshotMessagetoPeer(idForNetwork, s.PayloadHash(), PeerMessageTypeSnapshotAnnoucement, data)
}

func (me *Peer) SendSnapshotCommitmentMessage(idForNetwork crypto.Hash, snap crypto.Hash, R crypto.Key, wantTx bool) error {
	data := buildSnapshotCommitmentMessage(snap, R, wantTx)
	return me.sendSnapshotMessagetoPeer(idForNetwork, snap, PeerMessageTypeSnapshotCommitment, data)
}

func (me *Peer) SendTransactionChallengeMessage(idForNetwork crypto.Hash, snap crypto.Hash, cosi crypto.CosiSignature, tx *common.VersionedTransaction) error {
	data := buildTransactionChallengeMessage(snap, cosi, tx)
	return me.sendSnapshotMessagetoPeer(idForNetwork, snap, PeerMessageTypeTransactionChallenge, data)
}

func (me *Peer) SendSnapshotResponseMessage(idForNetwork crypto.Hash, snap crypto.Hash, si [32]byte) error {
	data := buildSnapshotResponseMessage(snap, si)
	return me.sendSnapshotMessagetoPeer(idForNetwork, snap, PeerMessageTypeSnapshotResponse, data)
}

func (me *Peer) SendSnapshotFinalizationMessage(idForNetwork crypto.Hash, s *common.Snapshot) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}

	hash := s.PayloadHash().ForNetwork(idForNetwork)
	key := crypto.NewHash(append(hash[:], 'S', 'C', 'O'))
	if me.snapshotsCaches.Exist(key, config.Custom.CacheTTL*time.Second/2) {
		return nil
	}

	data := buildSnapshotFinalizationMessage(s)
	return me.sendSnapshotMessagetoPeer(idForNetwork, s.PayloadHash(), PeerMessageTypeSnapshotFinalization, data)
}

func (me *Peer) SendSnapshotConfirmMessage(idForNetwork crypto.Hash, snap crypto.Hash) error {
	key := snap.ForNetwork(idForNetwork)
	key = crypto.NewHash(append(key[:], 'S', 'N', 'A', 'P', PeerMessageTypeSnapshotConfirm))
	return me.sendHighToPeer(idForNetwork, key, buildSnapshotConfirmMessage(snap))
}

func (me *Peer) SendTransactionRequestMessage(idForNetwork crypto.Hash, tx crypto.Hash) error {
	key := tx.ForNetwork(idForNetwork)
	key = crypto.NewHash(append(key[:], 'T', 'X', PeerMessageTypeTransactionRequest))
	return me.sendHighToPeer(idForNetwork, key, buildTransactionRequestMessage(tx))
}

func (me *Peer) SendTransactionMessage(idForNetwork crypto.Hash, ver *common.VersionedTransaction) error {
	key := ver.PayloadHash().ForNetwork(idForNetwork)
	key = crypto.NewHash(append(key[:], 'T', 'X', PeerMessageTypeTransaction))
	return me.sendHighToPeer(idForNetwork, key, buildTransactionMessage(ver))
}

func (me *Peer) ConfirmSnapshotForPeer(idForNetwork, snap crypto.Hash) {
	hash := snap.ForNetwork(idForNetwork)
	key := crypto.NewHash(append(hash[:], 'S', 'C', 'O'))
	me.snapshotsCaches.Store(key, time.Now())
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
	case PeerMessageTypeGraph:
		err := common.MsgpackUnmarshal(data[1:], &msg.FinalCache)
		if err != nil {
			return nil, err
		}
	case PeerMessageTypePing:
	case PeerMessageTypeAuthentication:
		msg.Data = data[1:]
	case PeerMessageTypeSnapshotConfirm:
		copy(msg.SnapshotHash[:], data[1:])
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

func buildSnapshotAnnouncementMessage(s *common.Snapshot) []byte {
	data := common.MsgpackMarshalPanic(s)
	return append([]byte{PeerMessageTypeSnapshotAnnoucement}, data...)
}

func buildSnapshotCommitmentMessage(snap crypto.Hash, R crypto.Key, wantTx bool) []byte {
	data := []byte{PeerMessageTypeSnapshotCommitment}
	data = append(data, snap[:]...)
	data = append(data, R[:]...)
	if wantTx {
		return append(data, byte(1))
	}
	return append(data, byte(0))
}

func buildTransactionChallengeMessage(snap crypto.Hash, cosi crypto.CosiSignature, tx *common.VersionedTransaction) []byte {
	mask := make([]byte, 8)
	binary.BigEndian.PutUint64(mask, cosi.Mask)
	data := []byte{PeerMessageTypeTransactionChallenge}
	data = append(data, snap[:]...)
	data = append(data, cosi.Signature[:]...)
	data = append(data, mask...)
	if tx != nil {
		pl := tx.Marshal()
		return append(data, pl...)
	}
	return data
}

func buildSnapshotResponseMessage(snap crypto.Hash, si [32]byte) []byte {
	data := []byte{PeerMessageTypeSnapshotResponse}
	data = append(data, snap[:]...)
	return append(data, si[:]...)
}

func buildSnapshotFinalizationMessage(s *common.Snapshot) []byte {
	data := common.MsgpackMarshalPanic(s)
	return append([]byte{PeerMessageTypeSnapshotFinalization}, data...)
}

func buildSnapshotConfirmMessage(snap crypto.Hash) []byte {
	return append([]byte{PeerMessageTypeSnapshotConfirm}, snap[:]...)
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
	receive := make(chan *PeerMessage, 1024*16)

	defer func() {
		client.Close()
		done <- true
	}()

	peer, err := me.authenticateNeighbor(client)
	if err != nil {
		logger.Println("peer authentication error", client.RemoteAddr().String(), err)
		return err
	}

	go me.handlePeerMessage(peer, receive, done)

	for {
		data, err := client.Receive()
		if err != nil {
			return fmt.Errorf("client.Receive %s %s", peer.IdForNetwork, err.Error())
		}
		msg, err := parseNetworkMessage(data)
		if err != nil {
			return fmt.Errorf("parseNetworkMessage %s %s", peer.IdForNetwork, err.Error())
		}
		select {
		case receive <- msg:
		case <-time.After(1 * time.Second):
			return fmt.Errorf("peer receive timeout %s", peer.IdForNetwork)
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
			case PeerMessageTypeGraph:
				me.handle.UpdateSyncPoint(peer.IdForNetwork, msg.FinalCache)
				peer.sync <- msg.FinalCache
			case PeerMessageTypeTransactionRequest:
				me.handle.SendTransactionToPeer(peer.IdForNetwork, msg.TransactionHash)
			case PeerMessageTypeTransaction:
				me.handle.CachePutTransaction(peer.IdForNetwork, msg.Transaction)
			case PeerMessageTypeSnapshotConfirm:
				me.ConfirmSnapshotForPeer(peer.IdForNetwork, msg.SnapshotHash)
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

		peer = me.neighbors[id]
		add, err := me.AddNeighbor(id, addr)
		if err == nil {
			peer = add
		}
		if peer == nil {
			auth <- errors.New("peer authentication message signature invalid")
		} else {
			auth <- nil
		}
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

func (me *Peer) sendHighToPeer(idForNetwork, key crypto.Hash, data []byte) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	peer := me.neighbors[idForNetwork]
	if peer == nil {
		return nil
	}
	if me.snapshotsCaches.Exist(key, time.Minute) {
		return nil
	}

	select {
	case peer.high <- &ChanMsg{key, data}:
		return nil
	case <-time.After(1 * time.Second):
		return errors.New("peer send high timeout")
	}
}

func (me *Peer) sendSnapshotMessagetoPeer(idForNetwork crypto.Hash, snap crypto.Hash, typ byte, data []byte) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	peer := me.neighbors[idForNetwork]
	if peer == nil {
		return nil
	}
	hash := snap.ForNetwork(idForNetwork)
	key := crypto.NewHash(append(hash[:], 'S', 'N', 'A', 'P', typ))
	if me.snapshotsCaches.Exist(key, time.Minute) {
		return nil
	}

	select {
	case peer.normal <- &ChanMsg{key, data}:
		return nil
	case <-time.After(1 * time.Second):
		return errors.New("peer send normal timeout")
	}
}

type confirmMap struct {
	cache *fastcache.Cache
}

func (m *confirmMap) Exist(key crypto.Hash, duration time.Duration) bool {
	val := m.cache.Get(nil, key[:])
	if len(val) == 8 {
		ts := time.Unix(0, int64(binary.BigEndian.Uint64(val)))
		return ts.Add(duration).After(time.Now())
	}
	return false
}

func (m *confirmMap) Store(key crypto.Hash, ts time.Time) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(ts.UnixNano()))
	m.cache.Set(key[:], buf)
}
