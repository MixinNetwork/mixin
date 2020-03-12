package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/VictoriaMetrics/fastcache"
)

type Peer struct {
	IdForNetwork crypto.Hash
	Address      string

	storeCache      *fastcache.Cache
	snapshotsCaches *confirmMap
	neighbors       *neighborMap
	handle          SyncHandle
	transport       Transport
	high            chan *ChanMsg
	normal          chan *ChanMsg
	sync            chan []*SyncPoint
	closing         bool
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

func (me *Peer) AddNeighbor(idForNetwork crypto.Hash, addr string) (*Peer, error) {
	if a, err := net.ResolveUDPAddr("udp", addr); err != nil {
		return nil, fmt.Errorf("invalid address %s %s", addr, err)
	} else if a.Port < 80 || a.IP == nil {
		return nil, fmt.Errorf("invalid address %s %d %s", addr, a.Port, a.IP)
	}
	old := me.neighbors.Get(idForNetwork)
	if old != nil && old.Address == addr {
		return old, nil
	} else if old != nil {
		old.closing = true
	}

	peer := NewPeer(nil, idForNetwork, addr)
	me.neighbors.Set(idForNetwork, peer)
	go me.openPeerStreamLoop(peer)
	go me.syncToNeighborLoop(peer)
	return peer, nil
}

func NewPeer(handle SyncHandle, idForNetwork crypto.Hash, addr string) *Peer {
	peer := &Peer{
		IdForNetwork: idForNetwork,
		Address:      addr,
		neighbors:    &neighborMap{m: make(map[crypto.Hash]*Peer)},
		high:         make(chan *ChanMsg, 1024*1024),
		normal:       make(chan *ChanMsg, 1024*1024),
		sync:         make(chan []*SyncPoint, 1024*1024),
		handle:       handle,
	}
	if handle != nil {
		peer.storeCache = handle.GetCacheStore()
		peer.snapshotsCaches = &confirmMap{cache: peer.storeCache}
	}
	return peer
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
				logger.Debugf("accept neighbor error %s\n", err.Error())
			}
		}(c)
	}
}

func (me *Peer) openPeerStreamLoop(p *Peer) {
	var resend *ChanMsg
	for !p.closing {
		msg, err := me.openPeerStream(p, resend)
		if err != nil {
			logger.Debugf("neighbor open stream error %s\n", err.Error())
		}
		resend = msg
		time.Sleep(1 * time.Second)
	}
}

func (me *Peer) openPeerStream(peer *Peer, resend *ChanMsg) (*ChanMsg, error) {
	logger.Debugf("OPEN PEER STREAM %s\n", peer.Address)
	transport, err := NewQuicClient(peer.Address)
	if err != nil {
		return nil, err
	}
	client, err := transport.Dial()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	logger.Debugf("DIAL PEER STREAM %s\n", peer.Address)

	err = client.Send(buildAuthenticationMessage(me.handle.BuildAuthenticationMessage()))
	if err != nil {
		return nil, err
	}
	logger.Debugf("AUTH PEER STREAM %s\n", peer.Address)

	graphTicker := time.NewTicker(time.Duration(config.SnapshotRoundGap / 2))
	defer graphTicker.Stop()

	if resend != nil {
		logger.Debugf("RESEND PEER STREAM %s\n", resend.key.String())
		if !me.snapshotsCaches.contains(resend.key, time.Minute) {
			err := client.Send(resend.data)
			if err != nil {
				return resend, err
			}
			me.snapshotsCaches.store(resend.key, time.Now())
		}
	}

	logger.Debugf("LOOP PEER STREAM %s\n", peer.Address)
	for !peer.closing {
		gd, hd, nd := false, false, false
		select {
		case <-graphTicker.C:
			err := client.Send(buildGraphMessage(me.handle.BuildGraph()))
			if err != nil {
				return nil, err
			}
		default:
			gd = true
		}

		select {
		case msg := <-peer.high:
			if !me.snapshotsCaches.contains(msg.key, time.Minute) {
				err := client.Send(msg.data)
				if err != nil {
					return msg, err
				}
				me.snapshotsCaches.store(msg.key, time.Now())
			}
		default:
			hd = true
		}

		select {
		case msg := <-peer.normal:
			if !me.snapshotsCaches.contains(msg.key, time.Minute) {
				err := client.Send(msg.data)
				if err != nil {
					return msg, err
				}
				me.snapshotsCaches.store(msg.key, time.Now())
			}
		default:
			nd = true
		}

		if gd && hd && nd {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil, fmt.Errorf("PEER DONE")
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
		logger.Debugf("peer authentication error %s %s\n", client.RemoteAddr().String(), err.Error())
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
		timer := time.NewTimer(1 * time.Second)
		select {
		case receive <- msg:
		case <-timer.C:
			return fmt.Errorf("peer receive timeout %s", peer.IdForNetwork)
		}
		timer.Stop()
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
			auth <- fmt.Errorf("peer authentication invalid message type %d", msg.Type)
			return
		}

		id, addr, err := me.handle.Authenticate(msg.Auth)
		if err != nil {
			auth <- err
			return
		}

		peer = me.neighbors.Get(id) // FIXME deprecate this
		add, err := me.AddNeighbor(id, addr)
		if err == nil {
			peer = add
		}
		if peer == nil {
			auth <- fmt.Errorf("peer authentication add neighbor failed %s", err.Error())
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
		return nil, fmt.Errorf("peer authentication timeout")
	}
	return peer, nil
}

func (me *Peer) sendHighToPeer(idForNetwork, key crypto.Hash, data []byte) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	peer := me.neighbors.Get(idForNetwork)
	if peer == nil {
		return nil
	}
	if me.snapshotsCaches.contains(key, time.Minute) {
		return nil
	}

	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()
	select {
	case peer.high <- &ChanMsg{key, data}:
		return nil
	case <-timer.C:
		return fmt.Errorf("peer send high timeout")
	}
}

func (me *Peer) sendSnapshotMessageToPeer(idForNetwork crypto.Hash, snap crypto.Hash, typ byte, data []byte) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	peer := me.neighbors.Get(idForNetwork)
	if peer == nil {
		return nil
	}
	hash := snap.ForNetwork(idForNetwork)
	key := crypto.NewHash(append(hash[:], 'S', 'N', 'A', 'P', typ))
	if me.snapshotsCaches.contains(key, time.Minute) {
		return nil
	}

	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()
	select {
	case peer.normal <- &ChanMsg{key, data}:
		return nil
	case <-timer.C:
		return fmt.Errorf("peer send normal timeout")
	}
}

type confirmMap struct {
	cache *fastcache.Cache
}

func (m *confirmMap) contains(key crypto.Hash, duration time.Duration) bool {
	val := m.cache.Get(nil, key[:])
	if len(val) == 8 {
		ts := time.Unix(0, int64(binary.BigEndian.Uint64(val)))
		return ts.Add(duration).After(time.Now())
	}
	return false
}

func (m *confirmMap) store(key crypto.Hash, ts time.Time) {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(ts.UnixNano()))
	m.cache.Set(key[:], buf)
}

type neighborMap struct {
	sync.RWMutex
	m map[crypto.Hash]*Peer
}

func (m *neighborMap) Get(key crypto.Hash) *Peer {
	m.RLock()
	defer m.RUnlock()

	return m.m[key]
}

func (m *neighborMap) Set(key crypto.Hash, v *Peer) {
	m.Lock()
	defer m.Unlock()

	m.m[key] = v
}
