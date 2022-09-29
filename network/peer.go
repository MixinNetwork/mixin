package network

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/util"
	"github.com/dgraph-io/ristretto"
)

type Peer struct {
	IdForNetwork crypto.Hash
	Address      string

	ctx             context.Context
	snapshotsCaches *confirmMap
	neighbors       *neighborMap
	gossipRound     *neighborMap
	pingFilter      *neighborMap
	handle          SyncHandle
	transport       Transport
	gossipNeighbors bool
	highRing        *util.RingBuffer
	normalRing      *util.RingBuffer
	syncRing        *util.RingBuffer
	closing         bool
	ops             chan struct{}
	stn             chan struct{}
}

type SyncPoint struct {
	NodeId crypto.Hash `json:"node"`
	Number uint64      `json:"round"`
	Hash   crypto.Hash `json:"hash"`
	Pool   interface{} `json:"pool" msgpack:"-"`
}

type ChanMsg struct {
	key  []byte
	data []byte
}

func (me *Peer) PingNeighbor(addr string) error {
	if a, err := net.ResolveUDPAddr("udp", addr); err != nil {
		return fmt.Errorf("invalid address %s %s", addr, err)
	} else if a.Port < 80 || a.IP == nil {
		return fmt.Errorf("invalid address %s %d %s", addr, a.Port, a.IP)
	}
	key := crypto.NewHash([]byte(addr))
	if me.pingFilter.Get(key) != nil {
		return nil
	}
	me.pingFilter.Set(key, &Peer{})

	go func() {
		for !me.closing {
			err := me.pingPeerStream(addr)
			if err != nil {
				logger.Verbosef("PingNeighbor error %v\n", err)
			}
		}
	}()
	return nil
}

func (me *Peer) pingPeerStream(addr string) error {
	logger.Verbosef("PING OPEN PEER STREAM %s\n", addr)
	transport, err := NewQuicClient(addr)
	if err != nil {
		return err
	}
	client, err := transport.Dial(me.ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	logger.Verbosef("PING DIAL PEER STREAM %s\n", addr)

	err = client.Send(buildAuthenticationMessage(me.handle.BuildAuthenticationMessage()))
	if err != nil {
		return err
	}
	logger.Verbosef("PING AUTH PEER STREAM %s\n", addr)
	time.Sleep(time.Duration(config.SnapshotRoundGap))
	return nil
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
		old.disconnect()
	}

	peer := NewPeer(nil, idForNetwork, addr, false)
	me.neighbors.Set(idForNetwork, peer)
	go me.openPeerStreamLoop(peer)
	go me.syncToNeighborLoop(peer)
	return peer, nil
}

func (me *Peer) Neighbors() []*Peer {
	return me.neighbors.Slice()
}

func (p *Peer) disconnect() {
	p.closing = true
	p.highRing.Dispose()
	p.normalRing.Dispose()
	p.syncRing.Dispose()
	<-p.ops
	<-p.stn
}

func NewPeer(handle SyncHandle, idForNetwork crypto.Hash, addr string, gossipNeighbors bool) *Peer {
	peer := &Peer{
		IdForNetwork:    idForNetwork,
		Address:         addr,
		neighbors:       &neighborMap{m: make(map[crypto.Hash]*Peer)},
		gossipRound:     &neighborMap{m: make(map[crypto.Hash]*Peer)},
		pingFilter:      &neighborMap{m: make(map[crypto.Hash]*Peer)},
		gossipNeighbors: gossipNeighbors,
		highRing:        util.NewRingBuffer(1024),
		normalRing:      util.NewRingBuffer(1024),
		syncRing:        util.NewRingBuffer(1024),
		handle:          handle,
		ops:             make(chan struct{}),
		stn:             make(chan struct{}),
	}
	peer.ctx = context.Background() // FIXME use real context
	if handle != nil {
		peer.snapshotsCaches = &confirmMap{cache: handle.GetCacheStore()}
	}
	return peer
}

func (me *Peer) Teardown() {
	me.closing = true
	me.transport.Close()
	me.highRing.Dispose()
	me.normalRing.Dispose()
	me.syncRing.Dispose()
	neighbors := me.neighbors.Slice()
	var wg sync.WaitGroup
	for _, p := range neighbors {
		wg.Add(1)
		go func(p *Peer) {
			p.disconnect()
			wg.Done()
		}(p)
	}
	wg.Wait()
	logger.Printf("Teardown(%s, %s)\n", me.IdForNetwork, me.Address)
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

	go func() {
		ticker := time.NewTicker(time.Duration(config.SnapshotRoundGap))
		defer ticker.Stop()

		for !me.closing {
			me.gossipRound.Clear()
			rand.Seed(time.Now().UnixNano())
			neighbors := me.neighbors.Slice()
			for i := range neighbors {
				j := rand.Intn(i + 1)
				neighbors[i], neighbors[j] = neighbors[j], neighbors[i]
			}
			if len(neighbors) > config.GossipSize {
				neighbors = neighbors[:config.GossipSize]
			}
			for _, p := range neighbors {
				me.gossipRound.Set(p.IdForNetwork, p)
			}

			<-ticker.C
		}
	}()

	for !me.closing {
		c, err := me.transport.Accept(me.ctx)
		if err != nil {
			logger.Verbosef("accept error %v\n", err)
			continue
		}
		go func(c Client) {
			err := me.acceptNeighborConnection(c)
			if err != nil {
				logger.Debugf("accept neighbor %s error %v\n", c.RemoteAddr().String(), err)
			}
		}(c)
	}

	logger.Printf("ListenNeighbors(%s, %s) DONE\n", me.IdForNetwork, me.Address)
	return nil
}

func (me *Peer) openPeerStreamLoop(p *Peer) {
	defer close(p.ops)

	var resend *ChanMsg
	for !me.closing && !p.closing {
		msg, err := me.openPeerStream(p, resend)
		if err != nil {
			logger.Verbosef("neighbor open stream %s error %v\n", p.Address, err)
		}
		resend = msg
		time.Sleep(1 * time.Second)
	}
}

func (me *Peer) openPeerStream(p *Peer, resend *ChanMsg) (*ChanMsg, error) {
	logger.Verbosef("OPEN PEER STREAM %s\n", p.Address)
	transport, err := NewQuicClient(p.Address)
	if err != nil {
		return nil, err
	}
	client, err := transport.Dial(me.ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	logger.Verbosef("DIAL PEER STREAM %s\n", p.Address)

	err = client.Send(buildAuthenticationMessage(me.handle.BuildAuthenticationMessage()))
	if err != nil {
		return nil, err
	}
	logger.Verbosef("AUTH PEER STREAM %s\n", p.Address)

	if resend != nil && !me.snapshotsCaches.contains(resend.key, time.Minute) {
		logger.Verbosef("RESEND PEER STREAM %s\n", hex.EncodeToString(resend.key))
		err := client.Send(resend.data)
		if err != nil {
			return resend, err
		}
		if resend.key != nil {
			me.snapshotsCaches.store(resend.key, time.Now())
		}
	}
	logger.Verbosef("LOOP PEER STREAM %s\n", p.Address)

	graphTicker := time.NewTicker(time.Duration(config.SnapshotRoundGap / 2))
	defer graphTicker.Stop()

	gossipNeighborsTicker := time.NewTicker(time.Duration(config.SnapshotRoundGap * 100))
	defer gossipNeighborsTicker.Stop()

	for !me.closing && !p.closing {
		msgs, size := []*ChanMsg{}, 0
		select {
		case <-graphTicker.C:
			msg := buildGraphMessage(me.handle.BuildGraph())
			msgs = append(msgs, &ChanMsg{nil, msg})
			size = size + len(msg)
		case <-gossipNeighborsTicker.C:
			if me.gossipNeighbors {
				msg := buildGossipNeighborsMessage(me.neighbors.Slice())
				msgs = append(msgs, &ChanMsg{nil, msg})
				size = size + len(msg)
			}
		default:
		}

		for len(msgs) < MaxMessageBundleSize && size < TransportMessageMaxSize/2 {
			item, err := p.highRing.Poll(false)
			if err != nil {
				return nil, err
			} else if item == nil {
				break
			}
			msg := item.(*ChanMsg)
			if me.snapshotsCaches.contains(msg.key, time.Minute) {
				continue
			}
			msgs = append(msgs, msg)
			size = size + len(msg.data)
		}

		for len(msgs) < MaxMessageBundleSize && size < TransportMessageMaxSize/2 {
			item, err := p.normalRing.Poll(false)
			if err != nil {
				return nil, err
			} else if item == nil {
				break
			}
			msg := item.(*ChanMsg)
			if me.snapshotsCaches.contains(msg.key, time.Minute) {
				continue
			}
			msgs = append(msgs, msg)
			size = size + len(msg.data)
		}

		if len(msgs) == 0 {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if len(msgs) == 1 {
			if msgs[0].key != nil && me.snapshotsCaches.contains(msgs[0].key, time.Minute) {
				continue
			}
			err := client.Send(msgs[0].data)
			if err != nil {
				return msgs[0], err
			}
			if msgs[0].key != nil {
				me.snapshotsCaches.store(msgs[0].key, time.Now())
			}
		} else {
			data := buildBundleMessage(msgs)
			err := client.Send(data)
			if err != nil {
				key := crypto.NewHash(data)
				return &ChanMsg{key[:], data}, err
			}
			for _, msg := range msgs {
				if msg.key != nil {
					me.snapshotsCaches.store(msg.key, time.Now())
				}
			}
		}
	}

	return nil, fmt.Errorf("PEER DONE")
}

func (me *Peer) acceptNeighborConnection(client Client) error {
	receive := make(chan *PeerMessage, 1024)

	defer func() {
		client.Close()
		close(receive)
	}()

	peer, err := me.authenticateNeighbor(client)
	if err != nil {
		return fmt.Errorf("peer authentication error %v", err)
	}

	go me.handlePeerMessage(peer, receive)

	for {
		tm, err := client.Receive()
		if err != nil {
			return fmt.Errorf("client.Receive %s %v", peer.IdForNetwork, err)
		}
		msg, err := parseNetworkMessage(tm.Version, tm.Data)
		if err != nil {
			return fmt.Errorf("parseNetworkMessage %s %v", peer.IdForNetwork, err)
		}

		if msg.Type != PeerMessageTypeBundle {
			select {
			case receive <- msg:
			default:
				return fmt.Errorf("peer receive timeout %s", peer.IdForNetwork)
			}
		}

		for data := msg.Data; len(data) > 4; {
			size := binary.BigEndian.Uint32(data[:4])
			if size < 16 || int(size+4) > len(data) {
				return fmt.Errorf("parseNetworkMessage %s invalid bundle element size", peer.IdForNetwork)
			}
			elm, err := parseNetworkMessage(tm.Version, data[4:4+size])
			if err != nil {
				return fmt.Errorf("parseNetworkMessage %s %v", peer.IdForNetwork, err)
			}
			if elm.Type == PeerMessageTypeBundle {
				return fmt.Errorf("parseNetworkMessage %s invalid bundle element type", peer.IdForNetwork)
			}
			select {
			case receive <- elm:
			default:
				return fmt.Errorf("peer receive timeout %s", peer.IdForNetwork)
			}
			data = data[4+size:]
		}
	}
}

func (me *Peer) authenticateNeighbor(client Client) (*Peer, error) {
	var peer *Peer
	auth := make(chan error)
	go func() {
		tm, err := client.Receive()
		if err != nil {
			auth <- err
			return
		}
		msg, err := parseNetworkMessage(tm.Version, tm.Data)
		if err != nil {
			auth <- err
			return
		}
		if msg.Type != PeerMessageTypeAuthentication {
			auth <- fmt.Errorf("peer authentication invalid message type %d", msg.Type)
			return
		}

		id, addr, err := me.handle.Authenticate(msg.Data)
		if err != nil {
			auth <- err
			return
		}

		peer, err = me.AddNeighbor(id, addr)
		if err != nil {
			auth <- fmt.Errorf("peer authentication add neighbor failed %v", err)
		} else {
			auth <- nil
		}
	}()

	select {
	case err := <-auth:
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("peer authentication failed %v", err)
		}
	case <-time.After(3 * time.Second):
		client.Close()
		return nil, fmt.Errorf("peer authentication timeout")
	}
	return peer, nil
}

func (me *Peer) sendHighToPeer(idForNetwork crypto.Hash, key, data []byte) error {
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

	success, _ := peer.highRing.Offer(&ChanMsg{key, data})
	if !success {
		return fmt.Errorf("peer send high timeout")
	}
	return nil
}

func (me *Peer) sendSnapshotMessageToPeer(idForNetwork crypto.Hash, snap crypto.Hash, typ byte, data []byte) error {
	if idForNetwork == me.IdForNetwork {
		return nil
	}
	peer := me.neighbors.Get(idForNetwork)
	if peer == nil {
		return nil
	}
	key := append(idForNetwork[:], snap[:]...)
	key = append(key, 'S', 'N', 'A', 'P', typ)
	if me.snapshotsCaches.contains(key, time.Minute) {
		return nil
	}

	success, _ := peer.normalRing.Offer(&ChanMsg{key, data})
	if !success {
		return fmt.Errorf("peer send normal timeout")
	}
	return nil
}

type confirmMap struct {
	cache *ristretto.Cache
}

func (m *confirmMap) contains(key []byte, duration time.Duration) bool {
	val, found := m.cache.Get(key)
	if found {
		ts := time.Unix(0, int64(binary.BigEndian.Uint64(val.([]byte))))
		return ts.Add(duration).After(time.Now())
	}
	return false
}

func (m *confirmMap) store(key []byte, ts time.Time) {
	if key == nil {
		panic(ts)
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(ts.UnixNano()))
	m.cache.Set(key, buf, 8)
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

func (m *neighborMap) Slice() []*Peer {
	m.Lock()
	defer m.Unlock()

	var peers []*Peer
	for _, p := range m.m {
		peers = append(peers, p)
	}
	return peers
}

func (m *neighborMap) Clear() {
	m.Lock()
	defer m.Unlock()

	for id := range m.m {
		delete(m.m, id)
	}
}
